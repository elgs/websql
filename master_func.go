// master_func
package websql

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

func (this *WebSQL) processWsCommandMaster(conn *websocket.Conn, message []byte) error {
	wsCommand := &Command{}
	json.Unmarshal(message, wsCommand)

	if wsCommand.Secret != Websql.service.Secret {
		regCommand := &Command{
			Type: "WS_REGISTER",
			Data: "Failed to valid client secret.",
		}
		conn.WriteJSON(regCommand)
		return errors.New(regCommand.Data)
	}

	switch wsCommand.Type {
	case "WS_REGISTER":
		//		log.Println(string(message))

		apiNode := &ApiNode{
			Id:   wsCommand.Data,
			Name: conn.RemoteAddr().String(),
		}
		err := AddApiNode(apiNode)
		if err != nil {
			conn.Close()
			return err
		}

		Websql.wsConns[apiNode.Id] = conn
		regCommand := &Command{
			Type: "WS_REGISTER",
			Data: "OK",
		}
		conn.WriteJSON(regCommand)
		log.Println(conn.RemoteAddr(), "connected.")

		masterDataBytes, err := json.Marshal(this.masterData)
		if err != nil {
			conn.Close()
			return err
		}
		masterDataCommand := &Command{
			Type: "WS_MASTER_DATA",
			Data: string(masterDataBytes),
		}
		err = conn.WriteJSON(masterDataCommand)
		if err != nil {
			conn.Close()
			return err
		}
		log.Println(conn.RemoteAddr(), "master data sent.")
	}
	return nil
}

var masterDataMutex = &sync.Mutex{}

func (this *MasterData) Propagate() error {
	masterDataMutex.Lock()
	var err error
	masterDataBytes, err := json.Marshal(this)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(Websql.service.DataFile, masterDataBytes, 0644)
	if err != nil {
		return err
	}
	masterDataMutex.Unlock()
	masterDataCommand := &Command{
		Type: "WS_MASTER_DATA",
		Data: string(masterDataBytes),
	}
	for _, conn := range Websql.wsConns {
		err = conn.WriteJSON(masterDataCommand)
		if err != nil {
			log.Println(err)
		}
	}
	return err
}
