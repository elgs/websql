package websql

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"

	"github.com/elgs/cron"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
	"github.com/urfave/cli"
)

var Websql *WebSQL = &WebSQL{
	AppName:        "websql",
	AppDescription: "An SQL backend for the web.",
	AppVersion:     "0.0.1",
	Interceptors: &Interceptors{
		GlobalDataInterceptorRegistry:    map[int]DataInterceptor{},
		DataInterceptorRegistry:          map[string]map[int]DataInterceptor{},
		GlobalHandlerInterceptorRegistry: []HandlerInterceptor{},
		HandlerInterceptorRegistry:       map[string]HandlerInterceptor{},
	},
	handlers: &Handlers{
		handlerRegistry: make(map[string]func(w http.ResponseWriter, r *http.Request)),
		DboRegistry:     make(map[string]DataOperator),
	},
	service: &CliService{
		EnableHttp: true,
		HttpHost:   "127.0.0.1",
	},
	wsConns:    make(map[string]*websocket.Conn),
	jobStatus:  make(map[string]int),
	masterData: &MasterData{},
	Sched:      cron.New(),
}

type WebSQL struct {
	AppName        string
	AppDescription string
	AppVersion     string
	slaveConn      *websocket.Conn
	wsConns        map[string]*websocket.Conn
	masterData     *MasterData
	apiNodes       []*ApiNode
	service        *CliService
	Sched          *cron.Cron
	jobStatus      map[string]int
	Interceptors   *Interceptors
	handlers       *Handlers
	getDbo         func(id string) (DataOperator, error)
}

//var slaveConn *websocket.Conn
//var wsConns = make(map[string]*websocket.Conn)
//var masterData MasterData
//var apiNodes []*ApiNode
var pwd string
var homeDir string

//var service = &CliService{
//	EnableHttp: true,
//	HttpHost:   "127.0.0.1",
//}

func Run(appName string, appVersion string) {
	Websql.AppName = appName
	Websql.AppVersion = appVersion
	// read config file
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	homeDir = usr.HomeDir
	pwd, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	sigs := make(chan os.Signal, 1)
	wsDrop := make(chan bool, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			select {
			case sig := <-sigs:
				fmt.Println()
				fmt.Println(sig)
				// cleanup code here
				done <- true
			case <-wsDrop:
				RegisterToMaster(wsDrop)
			}
		}
	}()

	app := cli.NewApp()
	app.Name = Websql.AppName
	app.Usage = Websql.AppDescription
	app.Version = Websql.AppVersion

	app.Commands = []cli.Command{
		{
			Name:    "service",
			Aliases: []string{"s"},
			Usage:   "service commands",
			Subcommands: []cli.Command{
				{
					Name:    "start",
					Aliases: []string{"s"},
					Usage:   "start service",
					Flags:   Websql.service.Flags(),
					Action: func(c *cli.Context) error {
						Websql.service.LoadConfigs(c)

						Websql.getDbo = MakeGetDbo("mysql", Websql.masterData)

						if len(strings.TrimSpace(Websql.service.Master)) > 0 {
							// load data from master if slave
							if RegisterToMaster(wsDrop) != nil {
								return err
							}
						} else {
							// load data from data file if master
							if _, err := os.Stat(Websql.service.DataFile); os.IsNotExist(err) {
								fmt.Println(err)
							} else {
								masterDataBytes, err := ioutil.ReadFile(Websql.service.DataFile)
								if err != nil {
									return err
								}
								err = json.Unmarshal(masterDataBytes, Websql.masterData)
								if err != nil {
									return err
								}
							}

							Websql.StartJobs()
							Websql.handlers.RegisterHandler("/sys/ws", func(w http.ResponseWriter, r *http.Request) {
								conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
								if err != nil {
									http.Error(w, err.Error(), http.StatusInternalServerError)
									return
								}
								go func(c *websocket.Conn) {
									defer c.Close()
									for {
										_, message, err := c.ReadMessage()
										if err != nil {
											err = RemoveApiNode(c.RemoteAddr().String())
											if err != nil {
												log.Println(err)
											}
											err = c.Close()
											if err != nil {
												log.Println(err)
											}
											log.Println(c.RemoteAddr(), "dropped.")
											for k, v := range Websql.wsConns {
												if v == c {
													delete(Websql.wsConns, k)
													break
												}
											}
											break
										}
										// Master to process command from client web socket channels.
										err = Websql.processWsCommandMaster(c, message)
										if err != nil {
											log.Println(err)
										}
									}
								}(conn)
							})
						}
						// shutdown
						Websql.handlers.RegisterHandler("/sys/shutdown", func(w http.ResponseWriter, r *http.Request) {
							if strings.HasPrefix(r.RemoteAddr, "127.0.0.1:") {
								done <- true
							} else {
								fmt.Fprintln(w, "Attack!!!")
							}
						})
						// cli
						Websql.handlers.RegisterHandler("/sys/cli", func(w http.ResponseWriter, r *http.Request) {
							res, err := ioutil.ReadAll(r.Body)
							if err != nil {
								fmt.Fprint(w, err.Error())
								return
							}
							if Websql.service.Master == "" {
								// Master to process commands from cli interface.
								//								log.Println("I'm master")
								result, err := Websql.processCliCommand(res)
								if err != nil {
									fmt.Fprint(w, err.Error())
									return
								}
								fmt.Fprint(w, result)
							} else {
								//								log.Println("I'm slave")
								cliCommand := &Command{}
								json.Unmarshal(res, cliCommand)
								// Slave to forward cli command to master.
								response, err := sendCliCommand(Websql.service.Master, cliCommand, false)
								if err != nil {
									fmt.Fprint(w, err.Error())
									return
								}
								output := string(response)
								fmt.Fprint(w, output)
							}
						})

						Websql.handlers.RegisterHandler("/api", RestFunc)

						// serve
						serve(Websql.service)
						<-done
						fmt.Println("Bye!")
						return nil
					},
				},
				{
					Name:    "stop",
					Aliases: []string{"st"},
					Usage:   "stop service",
					Action: func(c *cli.Context) error {
						if len(c.Args()) > 0 {
							_, err := http.Post(fmt.Sprint("http://127.0.0.1:", c.Args()[0], "/sys/shutdown"), "text/plain", nil)
							if err != nil {
								fmt.Println(err)
								return err
							}
						} else {
							fmt.Println("Usage: " + Websql.AppName + " service stop <shutdown_port>")
						}
						return nil
					},
				},
			},
		},
		{
			Name:    "datanode",
			Aliases: []string{"dn"},
			Usage:   "data node commands",
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "list all data nodes",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.BoolTFlag{
							Name:  "full, f",
							Usage: "show a full list of data nodes",
						},
						cli.BoolTFlag{
							Name:  "compact, c",
							Usage: "show a compact list of data nodes",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						full := c.IsSet("full")
						compact := c.IsSet("compact")
						mode := "normal"
						if compact {
							mode = "compact"
						} else if full {
							mode = "full"
						}
						cliDnListCommand := &Command{
							Type: "CLI_DN_LIST",
							Data: mode,
						}
						response, err := sendCliCommand(node, cliDnListCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "add",
					Usage: "add a new data node",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the data node",
						},
						cli.StringFlag{
							Name:  "host, H",
							Usage: "hostname of the data node",
						},
						cli.IntFlag{
							Name:  "port, P",
							Value: 3306,
							Usage: "port number of the data node",
						},
						cli.StringFlag{
							Name:  "user, u",
							Usage: "username of the data node",
						},
						cli.StringFlag{
							Name:  "pass, p",
							Usage: "password of the node",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "a note for the data node",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						//						id := strings.Replace(uuid.NewV4().String(), "-", "", -1)
						dataNode := &DataNode{
							//							Id:       id,
							Name:     c.String("name"),
							Host:     c.String("host"),
							Port:     c.Int("port"),
							Username: c.String("user"),
							Password: c.String("pass"),
							Note:     c.String("note"),
						}
						dataNodeJSONBytes, err := json.Marshal(dataNode)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliDnAddCommand := &Command{
							Type: "CLI_DN_ADD",
							Data: string(dataNodeJSONBytes),
						}
						response, err := sendCliCommand(node, cliDnAddCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "update",
					Usage: "update an existing data node",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the data node",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the data node",
						},
						cli.StringFlag{
							Name:  "host, H",
							Usage: "hostname of the data node",
						},
						cli.IntFlag{
							Name:  "port, P",
							Value: 3306,
							Usage: "port number of the data node",
						},
						cli.StringFlag{
							Name:  "user, u",
							Usage: "username of the data node",
						},
						cli.StringFlag{
							Name:  "pass, p",
							Usage: "password of the node",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "a note for the data node",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						dataNode := &DataNode{
							Id:       c.String("id"),
							Name:     c.String("name"),
							Host:     c.String("host"),
							Port:     c.Int("port"),
							Username: c.String("user"),
							Password: c.String("pass"),
							Note:     c.String("note"),
						}
						if !c.IsSet("name") {
							dataNode.Name = "__not_set__"
						}
						if !c.IsSet("host") {
							dataNode.Host = "__not_set__"
						}
						if !c.IsSet("port") {
							dataNode.Port = -1
						}
						if !c.IsSet("user") {
							dataNode.Username = "__not_set__"
						}
						if !c.IsSet("pass") {
							dataNode.Password = "__not_set__"
						}
						if !c.IsSet("note") {
							dataNode.Note = "__not_set__"
						}
						dataNodeJSONBytes, err := json.Marshal(dataNode)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliDnUpdateCommand := &Command{
							Type: "CLI_DN_UPDATE",
							Data: string(dataNodeJSONBytes),
						}
						response, err := sendCliCommand(node, cliDnUpdateCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "remove",
					Usage: "remove an existing data node",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the data node",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						id := c.String("id")
						cliDnRemoveCommand := &Command{
							Type: "CLI_DN_REMOVE",
							Data: id,
						}
						response, err := sendCliCommand(node, cliDnRemoveCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
			},
		},
		{
			Name:    "app",
			Aliases: []string{"a"},
			Usage:   "app commands",
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "list all apps",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.BoolTFlag{
							Name:  "full, f",
							Usage: "show a full list of apps",
						},
						cli.BoolTFlag{
							Name:  "compact, c",
							Usage: "show a compact list of apps",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						full := c.IsSet("full")
						compact := c.IsSet("compact")
						mode := "normal"
						if compact {
							mode = "compact"
						} else if full {
							mode = "full"
						}
						cliAppListCommand := &Command{
							Type: "CLI_APP_LIST",
							Data: mode,
						}
						response, err := sendCliCommand(node, cliAppListCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "add",
					Usage: "add a new app",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the app",
						},
						cli.StringFlag{
							Name:  "datanode, d",
							Usage: "data node id",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "a note for the app",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")

						name := c.String("name")
						//						namePrefix := name[:int(math.Min(float64(len(name)), 8))]

						//						id := strings.Replace(uuid.NewV4().String(), "-", "", -1)
						//						dbName, err := gostrgen.RandGen(16-len(namePrefix), gostrgen.LowerDigit, "", "")
						//						if err != nil {
						//							return err
						//						}

						app := &App{
							//							Id:         id,
							Name:       name,
							DataNodeId: c.String("datanode"),
							//							DbName:     namePrefix + dbName,
							Note: c.String("note"),
						}
						appJSONBytes, err := json.Marshal(app)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliAppAddCommand := &Command{
							Type: "CLI_APP_ADD",
							Data: string(appJSONBytes),
						}
						response, err := sendCliCommand(node, cliAppAddCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "update",
					Usage: "update an existing app",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the app",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the app",
						},
						cli.StringFlag{
							Name:  "datanode, d",
							Usage: "data node id",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "a note for the app",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						app := &App{
							Id:         c.String("id"),
							Name:       c.String("name"),
							DataNodeId: c.String("datanode"),
							Note:       c.String("note"),
						}
						if !c.IsSet("name") {
							app.Name = "__not_set__"
						}
						if !c.IsSet("datanode") {
							app.DataNodeId = "__not_set__"
						}
						if !c.IsSet("note") {
							app.Note = "__not_set__"
						}
						appJSONBytes, err := json.Marshal(app)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliAppUpdateCommand := &Command{
							Type: "CLI_APP_UPDATE",
							Data: string(appJSONBytes),
						}
						response, err := sendCliCommand(node, cliAppUpdateCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "remove",
					Usage: "remove an existing app",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the app",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						id := c.String("id")
						cliAppRemoveCommand := &Command{
							Type: "CLI_APP_REMOVE",
							Data: id,
						}
						response, err := sendCliCommand(node, cliAppRemoveCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
			},
		},
		{
			Name:    "query",
			Aliases: []string{"q"},
			Usage:   "query commands",
			Subcommands: []cli.Command{
				{
					Name:  "add",
					Usage: "add a new query",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the query",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "script, s",
							Usage: "script path of the query",
						},
						cli.StringFlag{
							Name:  "mode, o",
							Usage: "query mode, public or private",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "a note for the query",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						id := strings.Replace(uuid.NewV4().String(), "-", "", -1)
						query := &Query{
							Id:         id,
							Name:       c.String("name"),
							AppId:      c.String("app"),
							ScriptPath: c.String("script"),
							Mode:       c.String("mode"),
							Note:       c.String("note"),
						}
						queryJSONBytes, err := json.Marshal(query)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliQueryAddCommand := &Command{
							Type: "CLI_QUERY_ADD",
							Data: string(queryJSONBytes),
						}
						response, err := sendCliCommand(node, cliQueryAddCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "update",
					Usage: "update an existing query",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the query",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the query",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "script, s",
							Usage: "script path of the query",
						},
						cli.StringFlag{
							Name:  "mode, o",
							Usage: "query mode, public or private",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "a note for the query",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						query := &Query{
							Id:         c.String("id"),
							Name:       c.String("name"),
							AppId:      c.String("app"),
							ScriptPath: c.String("script"),
							Mode:       c.String("mode"),
							Note:       c.String("note"),
						}
						if !c.IsSet("name") {
							query.Name = "__not_set__"
						}
						if !c.IsSet("script") {
							query.ScriptPath = "__not_set__"
						}
						if !c.IsSet("mode") {
							query.Mode = "__not_set__"
						}
						if !c.IsSet("note") {
							query.Note = "__not_set__"
						}
						queryJSONBytes, err := json.Marshal(query)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliQueryUpdateCommand := &Command{
							Type: "CLI_QUERY_UPDATE",
							Data: string(queryJSONBytes),
						}
						response, err := sendCliCommand(node, cliQueryUpdateCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "reload",
					Usage: "reload all queries",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						appId := c.String("app")
						cliQueryReloadAllCommand := &Command{
							Type: "CLI_QUERY_RELOAD_ALL",
							Data: appId,
						}
						response, err := sendCliCommand(node, cliQueryReloadAllCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "remove",
					Usage: "remove an existing query",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the query",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						query := &Query{
							Id:    c.String("id"),
							AppId: c.String("app"),
						}
						queryJSONBytes, err := json.Marshal(query)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliQueryRemoveCommand := &Command{
							Type: "CLI_QUERY_REMOVE",
							Data: string(queryJSONBytes),
						}
						response, err := sendCliCommand(node, cliQueryRemoveCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
			},
		},
		{
			Name:    "job",
			Aliases: []string{"j"},
			Usage:   "job commands",
			Subcommands: []cli.Command{
				{
					Name:  "add",
					Usage: "add a new job",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the job",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "cron, c",
							Usage: "cron expression of the job",
						},
						cli.StringFlag{
							Name:  "script, s",
							Usage: "script path of the job",
						},
						cli.StringFlag{
							Name:  "loopscript, l",
							Usage: "loop script path of the job",
						},
						cli.IntFlag{
							Name:  "auto, u",
							Usage: "auto start the job?  0: no, 1: yes",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "a note for the job",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						id := strings.Replace(uuid.NewV4().String(), "-", "", -1)
						job := &Job{
							Id:             id,
							Name:           c.String("name"),
							AppId:          c.String("app"),
							ScriptPath:     c.String("script"),
							LoopScriptPath: c.String("loopscript"),
							Cron:           c.String("cron"),
							AutoStart:      c.Int("auto"),
							Note:           c.String("note"),
						}
						jobJSONBytes, err := json.Marshal(job)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliJobAddCommand := &Command{
							Type: "CLI_JOB_ADD",
							Data: string(jobJSONBytes),
						}
						response, err := sendCliCommand(node, cliJobAddCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "update",
					Usage: "update an existing job",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the job",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the job",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "cron, c",
							Usage: "cron expression of the job",
						},
						cli.StringFlag{
							Name:  "script, s",
							Usage: "script path of the job",
						},
						cli.StringFlag{
							Name:  "loopscript, l",
							Usage: "loop script path of the job",
						},
						cli.IntFlag{
							Name:  "auto, u",
							Usage: "auto start the job?  0: no, 1: yes",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "a note for the job",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						job := &Job{
							Id:             c.String("id"),
							Name:           c.String("name"),
							AppId:          c.String("app"),
							ScriptPath:     c.String("script"),
							LoopScriptPath: c.String("loopscript"),
							Cron:           c.String("cron"),
							AutoStart:      c.Int("auto"),
							Note:           c.String("note"),
						}
						if !c.IsSet("name") {
							job.Name = "__not_set__"
						}
						if !c.IsSet("script") {
							job.ScriptPath = "__not_set__"
						}
						if !c.IsSet("loopscript") {
							job.LoopScriptPath = "__not_set__"
						}
						if !c.IsSet("cron") {
							job.Cron = "__not_set__"
						}
						if !c.IsSet("auto") {
							job.AutoStart = -1
						}
						if !c.IsSet("note") {
							job.Note = "__not_set__"
						}
						jobJSONBytes, err := json.Marshal(job)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliJobUpdateCommand := &Command{
							Type: "CLI_JOB_UPDATE",
							Data: string(jobJSONBytes),
						}
						response, err := sendCliCommand(node, cliJobUpdateCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "remove",
					Usage: "remove an existing job",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the job",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						job := &Job{
							Id:    c.String("id"),
							AppId: c.String("app"),
						}
						jobJSONBytes, err := json.Marshal(job)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliJobRemoveCommand := &Command{
							Type: "CLI_JOB_REMOVE",
							Data: string(jobJSONBytes),
						}
						response, err := sendCliCommand(node, cliJobRemoveCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "start",
					Usage: "start a job",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the job",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						job := &Job{
							Id:    c.String("id"),
							AppId: c.String("app"),
						}
						jobJSONBytes, err := json.Marshal(job)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliJobStartCommand := &Command{
							Type: "CLI_JOB_START",
							Data: string(jobJSONBytes),
						}
						response, err := sendCliCommand(node, cliJobStartCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "restart",
					Usage: "restart a job",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the job",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						job := &Job{
							Id:    c.String("id"),
							AppId: c.String("app"),
						}
						jobJSONBytes, err := json.Marshal(job)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliJobRestartCommand := &Command{
							Type: "CLI_JOB_RESTART",
							Data: string(jobJSONBytes),
						}
						response, err := sendCliCommand(node, cliJobRestartCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "stop",
					Usage: "stop a job",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the job",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						job := &Job{
							Id:    c.String("id"),
							AppId: c.String("app"),
						}
						jobJSONBytes, err := json.Marshal(job)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliJobStopCommand := &Command{
							Type: "CLI_JOB_STOP",
							Data: string(jobJSONBytes),
						}
						response, err := sendCliCommand(node, cliJobStopCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
			},
		},
		{
			Name:    "token",
			Aliases: []string{"t"},
			Usage:   "token commands",
			Subcommands: []cli.Command{
				{
					Name:  "add",
					Usage: "add a new token",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the token",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "mode, o",
							Usage: "mode of the token",
						},
						cli.StringFlag{
							Name:  "target, g",
							Usage: "target of the token",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "note for the token",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						appId := c.String("app")
						id := appId + strings.Replace(uuid.NewV4().String(), "-", "", -1)
						token := &Token{
							Id:     id,
							Name:   c.String("name"),
							AppId:  appId,
							Mode:   c.String("mode"),
							Target: c.String("target"),
							Note:   c.String("note"),
						}
						tokenJSONBytes, err := json.Marshal(token)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliTokenAddCommand := &Command{
							Type: "CLI_TOKEN_ADD",
							Data: string(tokenJSONBytes),
						}
						response, err := sendCliCommand(node, cliTokenAddCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "update",
					Usage: "update an existing token",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the token",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the token",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "mode, o",
							Usage: "mode of the token",
						},
						cli.StringFlag{
							Name:  "target, g",
							Usage: "target of the token",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "a note for the token",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						token := &Token{
							Id:     c.String("id"),
							Name:   c.String("name"),
							AppId:  c.String("app"),
							Mode:   c.String("mode"),
							Target: c.String("target"),
							Note:   c.String("note"),
						}
						if !c.IsSet("name") {
							token.Name = "__not_set__"
						}
						if !c.IsSet("mode") {
							token.Mode = "__not_set__"
						}
						if !c.IsSet("target") {
							token.Target = "__not_set__"
						}
						if !c.IsSet("note") {
							token.Note = "__not_set__"
						}
						tokenJSONBytes, err := json.Marshal(token)
						if err != nil {
							fmt.Println(err)
							return err
						}
						cliTokenUpdateCommand := &Command{
							Type: "CLI_TOKEN_UPDATE",
							Data: string(tokenJSONBytes),
						}
						response, err := sendCliCommand(node, cliTokenUpdateCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "remove",
					Usage: "remove an existing token",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the token",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						token := &Token{
							Id:    c.String("id"),
							AppId: c.String("app"),
						}
						jobJSONBytes, err := json.Marshal(token)
						if err != nil {
							return err
						}
						cliTokenRemoveCommand := &Command{
							Type: "CLI_TOKEN_REMOVE",
							Data: string(jobJSONBytes),
						}
						response, err := sendCliCommand(node, cliTokenRemoveCommand, true)
						if err != nil {
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
			},
		},
		{
			Name:  "li",
			Usage: "local interceptor commands",
			Subcommands: []cli.Command{
				{
					Name:  "add",
					Usage: "add a new local interceptor",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the local interceptor",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "target, g",
							Usage: "target of the local interceptor",
						},
						cli.StringFlag{
							Name:  "callback, c",
							Usage: "callback query name for the local interceptor",
						},
						cli.StringFlag{
							Name:  "type, k",
							Usage: "type of the local interceptor",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "note for the local interceptor",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						id := strings.Replace(uuid.NewV4().String(), "-", "", -1)
						li := &LocalInterceptor{
							Id:       id,
							Name:     c.String("name"),
							AppId:    c.String("app"),
							Target:   c.String("target"),
							Callback: c.String("callback"),
							Type:     c.String("type"),
							Note:     c.String("note"),
						}
						liJSONBytes, err := json.Marshal(li)
						if err != nil {
							return err
						}
						cliLiAddCommand := &Command{
							Type: "CLI_LI_ADD",
							Data: string(liJSONBytes),
						}
						response, err := sendCliCommand(node, cliLiAddCommand, true)
						if err != nil {
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "update",
					Usage: "update an existing local interceptor",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the local interceptor",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the local interceptor",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "target, g",
							Usage: "target of the local interceptor",
						},
						cli.StringFlag{
							Name:  "callback, c",
							Usage: "callback query name for the local interceptor",
						},
						cli.StringFlag{
							Name:  "type, k",
							Usage: "type of the local interceptor",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "note for the local interceptor",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						li := &LocalInterceptor{
							Id:       c.String("id"),
							Name:     c.String("name"),
							AppId:    c.String("app"),
							Target:   c.String("target"),
							Callback: c.String("callback"),
							Type:     c.String("type"),
							Note:     c.String("note"),
						}
						if !c.IsSet("name") {
							li.Name = "__not_set__"
						}
						if !c.IsSet("target") {
							li.Target = "__not_set__"
						}
						if !c.IsSet("callback") {
							li.Callback = "__not_set__"
						}
						if !c.IsSet("type") {
							li.Type = "__not_set__"
						}
						if !c.IsSet("note") {
							li.Note = "__not_set__"
						}
						liJSONBytes, err := json.Marshal(li)
						if err != nil {
							return err
						}
						cliLiUpdateCommand := &Command{
							Type: "CLI_LI_UPDATE",
							Data: string(liJSONBytes),
						}
						response, err := sendCliCommand(node, cliLiUpdateCommand, true)
						if err != nil {
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "remove",
					Usage: "remove an existing local interceptor",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "the id of the local interceptor",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app name",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						li := &LocalInterceptor{
							Id:    c.String("id"),
							AppId: c.String("app"),
						}
						liJSONBytes, err := json.Marshal(li)
						if err != nil {
							return err
						}
						cliLiRemoveCommand := &Command{
							Type: "CLI_LI_REMOVE",
							Data: string(liJSONBytes),
						}
						response, err := sendCliCommand(node, cliLiRemoveCommand, true)
						if err != nil {
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
			},
		},
		{
			Name:  "ri",
			Usage: "remote interceptor commands",
			Subcommands: []cli.Command{
				{
					Name:  "add",
					Usage: "add a new remote interceptor",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "target, g",
							Usage: "target of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "method, e",
							Value: "POST",
							Usage: "method for the remote interceptor",
						},
						cli.StringFlag{
							Name:  "url, u",
							Usage: "url for the remote interceptor",
						},
						cli.StringFlag{
							Name:  "callback, c",
							Usage: "callback query name for the remote interceptor",
						},
						cli.StringFlag{
							Name:  "type, k",
							Usage: "type of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "action, o",
							Usage: "action type of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "note for the remote interceptor",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						id := strings.Replace(uuid.NewV4().String(), "-", "", -1)
						ri := &RemoteInterceptor{
							Id:         id,
							Name:       c.String("name"),
							AppId:      c.String("app"),
							Target:     c.String("target"),
							Method:     c.String("method"),
							Url:        c.String("url"),
							Callback:   c.String("callback"),
							Type:       c.String("type"),
							ActionType: c.String("action"),
							Note:       c.String("note"),
						}
						riJSONBytes, err := json.Marshal(ri)
						if err != nil {
							return err
						}
						cliRiAddCommand := &Command{
							Type: "CLI_RI_ADD",
							Data: string(riJSONBytes),
						}
						response, err := sendCliCommand(node, cliRiAddCommand, true)
						if err != nil {
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "update",
					Usage: "update an existing remote interceptor",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "id of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "name, n",
							Usage: "name of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:  "target, g",
							Usage: "target of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "method, e",
							Value: "POST",
							Usage: "method for the remote interceptor",
						},
						cli.StringFlag{
							Name:  "url, u",
							Usage: "url for the remote interceptor",
						},
						cli.StringFlag{
							Name:  "callback, c",
							Usage: "callback query name for the remote interceptor",
						},
						cli.StringFlag{
							Name:  "type, k",
							Usage: "type of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "action, i",
							Usage: "action type of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "note, t",
							Usage: "note for the remote interceptor",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						ri := &RemoteInterceptor{
							Id:         c.String("id"),
							Name:       c.String("name"),
							AppId:      c.String("app"),
							Target:     c.String("target"),
							Method:     c.String("method"),
							Url:        c.String("url"),
							Callback:   c.String("callback"),
							Type:       c.String("type"),
							ActionType: c.String("action"),
							Note:       c.String("note"),
						}
						if !c.IsSet("name") {
							ri.Name = "__not_set__"
						}
						if !c.IsSet("target") {
							ri.Target = "__not_set__"
						}
						if !c.IsSet("method") {
							ri.Method = "__not_set__"
						}
						if !c.IsSet("url") {
							ri.Url = "__not_set__"
						}
						if !c.IsSet("action") {
							ri.ActionType = "__not_set__"
						}
						if !c.IsSet("callback") {
							ri.Callback = "__not_set__"
						}
						if !c.IsSet("type") {
							ri.Type = "__not_set__"
						}
						if !c.IsSet("note") {
							ri.Note = "__not_set__"
						}
						riJSONBytes, err := json.Marshal(ri)
						if err != nil {
							return err
						}
						cliRiUpdateCommand := &Command{
							Type: "CLI_RI_UPDATE",
							Data: string(riJSONBytes),
						}
						response, err := sendCliCommand(node, cliRiUpdateCommand, true)
						if err != nil {
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "remove",
					Usage: "remove an existing remote interceptor",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:  "id, i",
							Usage: "the id of the remote interceptor",
						},
						cli.StringFlag{
							Name:  "app, a",
							Usage: "app id",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						ri := &RemoteInterceptor{
							Id:    c.String("id"),
							AppId: c.String("app"),
						}
						riJSONBytes, err := json.Marshal(ri)
						if err != nil {
							return err
						}
						cliRiRemoveCommand := &Command{
							Type: "CLI_RI_REMOVE",
							Data: string(riJSONBytes),
						}
						response, err := sendCliCommand(node, cliRiRemoveCommand, true)
						if err != nil {
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
			},
		},
		{
			Name:  "show",
			Usage: "show commands",
			Subcommands: []cli.Command{
				{
					Name:  "master",
					Usage: "show master data",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						cliShowMasterCommand := &Command{
							Type: "CLI_SHOW_MASTER",
						}
						response, err := sendCliCommand(node, cliShowMasterCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
				{
					Name:  "slave",
					Usage: "show slave api nodes info",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						cliShowApiNodesCommand := &Command{
							Type: "CLI_SHOW_API_NODES",
						}
						response, err := sendCliCommand(node, cliShowApiNodesCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
			},
		},
		{
			Name:  "master",
			Usage: "master commands",
			Subcommands: []cli.Command{
				{
					Name:    "propagate",
					Aliases: []string{"p"},
					Usage:   "propagate configuration data to all salves",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "node, N",
							Value: "127.0.0.1:2015",
							Usage: "node url, format: host:port. 127.0.0.1:2015 if empty",
						},
						cli.StringFlag{
							Name:        "secret, z",
							Usage:       "secret password for server client communication.",
							Destination: &Websql.service.Secret,
						},
					},
					Action: func(c *cli.Context) error {
						Websql.service.LoadSecrets(c)
						node := c.String("node")
						cliShowMasterCommand := &Command{
							Type: "CLI_PROPAGATE",
						}
						response, err := sendCliCommand(node, cliShowMasterCommand, true)
						if err != nil {
							fmt.Println(err)
							return err
						}
						output := string(response)
						if output != "" {
							fmt.Println(strings.TrimSpace(output))
						}
						return nil
					},
				},
			},
		},
	}
	err = app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
