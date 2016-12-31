// slave_func
package websql

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func sendCliCommand(node string, command *Command, attachSecret bool) ([]byte, error) {
	if attachSecret {
		command.Secret = Websql.service.Secret
	}
	message, err := json.Marshal(command)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("POST", "https://"+node+"/sys/cli", strings.NewReader(string(message)))
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	result, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return result, err
}

func RegisterToMaster(wsDrop chan bool) error {
	c, _, err := websocket.DefaultDialer.Dial("wss://"+Websql.service.Master+"/sys/ws", nil)
	if err != nil {
		log.Println(err)
		time.Sleep(time.Second * 5)
		wsDrop <- true
		return err
	}

	serviceBytes, err := json.Marshal(Websql.service)
	if err != nil {
		log.Println(err)
		time.Sleep(time.Second * 5)
		wsDrop <- true
		return err
	}
	regCommand := Command{
		Type: "WS_REGISTER",
		Data: string(serviceBytes),
	}

	// Register
	if err := c.WriteJSON(regCommand); err != nil {
		log.Println(err)
		time.Sleep(time.Second * 5)
		wsDrop <- true
		return err
	}
	var regResult string
	c.ReadJSON(&regResult)
	if regResult != "OK" {
		log.Println(regResult)
		return errors.New(regResult)
	}

	Websql.slaveConn = c
	go func() {
		defer c.Close()
		defer func() { wsDrop <- true }()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("Connection dropped. Reconnecting in 5 seconds...", err)
				time.Sleep(time.Second * 5)
				// Reconnect
				return
			}
			err = processWsCommandSlave(c, message)
			if err != nil {
				log.Println(err)
			}
		}
	}()

	log.Println("Connected to master:", Websql.service.Master)
	return nil
}

func processWsCommandSlave(conn *websocket.Conn, message []byte) error {
	wsCommand := &Command{}
	json.Unmarshal(message, wsCommand)
	switch wsCommand.Type {
	case "WS_MASTER_DATA":
		masterCommand := &Command{}
		err := json.Unmarshal(message, &masterCommand)
		if err != nil {
			return err
		}
		log.Println("Master data updated.")
		return json.Unmarshal([]byte(masterCommand.Data), Websql.masterData)
	}
	return nil
}
