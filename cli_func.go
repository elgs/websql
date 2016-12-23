// cli_func
package websql

import (
	"encoding/json"
	"errors"
)

func processCliCommand(message []byte) (string, error) {
	cliCommand := &Command{}
	json.Unmarshal(message, cliCommand)
	if service.Secret != cliCommand.Secret {
		return "", errors.New("Failed to validate secret.")
	}
	switch cliCommand.Type {
	case "CLI_DN_LIST":
		return masterData.ListDataNodes(cliCommand.Data), nil
	case "CLI_DN_ADD":
		dataNode := &DataNode{}
		err := json.Unmarshal([]byte(cliCommand.Data), dataNode)
		if err != nil {
			return "", err
		}
		err = masterData.AddDataNode(dataNode)
		if err != nil {
			return "", err
		}
	case "CLI_DN_UPDATE":
		dataNode := &DataNode{}
		err := json.Unmarshal([]byte(cliCommand.Data), dataNode)
		if err != nil {
			return "", err
		}
		err = masterData.UpdateDataNode(dataNode)
		if err != nil {
			return "", err
		}
	case "CLI_DN_REMOVE":
		err := masterData.RemoveDataNode(cliCommand.Data)
		if err != nil {
			return "", err
		}
	case "CLI_APP_LIST":
		return masterData.ListApps(cliCommand.Data), nil
	case "CLI_APP_ADD":
		app := &App{}
		err := json.Unmarshal([]byte(cliCommand.Data), app)
		if err != nil {
			return "", err
		}
		err = masterData.AddApp(app)
		if err != nil {
			return "", err
		}
	case "CLI_APP_UPDATE":
		app := &App{}
		err := json.Unmarshal([]byte(cliCommand.Data), app)
		if err != nil {
			return "", err
		}
		err = masterData.UpdateApp(app)
		if err != nil {
			return "", err
		}
	case "CLI_APP_REMOVE":
		err := masterData.RemoveApp(cliCommand.Data)
		if err != nil {
			return "", err
		}
	case "CLI_QUERY_ADD":
		query := &Query{}
		err := json.Unmarshal([]byte(cliCommand.Data), query)
		if err != nil {
			return "", err
		}
		err = masterData.AddQuery(query)
		if err != nil {
			return "", err
		}
	case "CLI_QUERY_UPDATE":
		query := &Query{}
		err := json.Unmarshal([]byte(cliCommand.Data), query)
		if err != nil {
			return "", err
		}
		err = masterData.UpdateQuery(query)
		if err != nil {
			return "", err
		}
	case "CLI_QUERY_RELOAD_ALL":
		err := masterData.ReloadAllQueries(cliCommand.Data)
		if err != nil {
			return "", err
		}
	case "CLI_QUERY_REMOVE":
		query := &Query{}
		err := json.Unmarshal([]byte(cliCommand.Data), query)
		if err != nil {
			return "", err
		}
		err = masterData.RemoveQuery(query.Id, query.AppId)
		if err != nil {
			return "", err
		}
	case "CLI_JOB_ADD":
		job := &Job{}
		err := json.Unmarshal([]byte(cliCommand.Data), job)
		if err != nil {
			return "", err
		}
		err = masterData.AddJob(job)
		if err != nil {
			return "", err
		}
	case "CLI_JOB_UPDATE":
		job := &Job{}
		err := json.Unmarshal([]byte(cliCommand.Data), job)
		if err != nil {
			return "", err
		}
		err = masterData.UpdateJob(job)
		if err != nil {
			return "", err
		}
	case "CLI_JOB_REMOVE":
		job := &Job{}
		err := json.Unmarshal([]byte(cliCommand.Data), job)
		if err != nil {
			return "", err
		}
		err = masterData.RemoveJob(job.Id, job.AppId)
		if err != nil {
			return "", err
		}
	case "CLI_JOB_START":
		job := &Job{}
		err := json.Unmarshal([]byte(cliCommand.Data), job)
		if err != nil {
			return "", err
		}
		err = masterData.StartJob(job)
		if err != nil {
			return "", err
		}
	case "CLI_JOB_RESTART":
		job := &Job{}
		err := json.Unmarshal([]byte(cliCommand.Data), job)
		if err != nil {
			return "", err
		}
		err = masterData.RestartJob(job)
		if err != nil {
			return "", err
		}
	case "CLI_JOB_STOP":
		job := &Job{}
		err := json.Unmarshal([]byte(cliCommand.Data), job)
		if err != nil {
			return "", err
		}
		err = masterData.StopJob(job)
		if err != nil {
			return "", err
		}
	case "CLI_TOKEN_ADD":
		token := &Token{}
		err := json.Unmarshal([]byte(cliCommand.Data), token)
		if err != nil {
			return "", err
		}
		err = masterData.AddToken(token)
		if err != nil {
			return "", err
		}
	case "CLI_TOKEN_UPDATE":
		token := &Token{}
		err := json.Unmarshal([]byte(cliCommand.Data), token)
		if err != nil {
			return "", err
		}
		err = masterData.UpdateToken(token)
		if err != nil {
			return "", err
		}
	case "CLI_TOKEN_REMOVE":
		token := &Token{}
		err := json.Unmarshal([]byte(cliCommand.Data), token)
		if err != nil {
			return "", err
		}
		err = masterData.RemoveToken(token.Id, token.AppId)
		if err != nil {
			return "", err
		}
	case "CLI_LI_ADD":
		li := &LocalInterceptor{}
		err := json.Unmarshal([]byte(cliCommand.Data), li)
		if err != nil {
			return "", err
		}
		err = masterData.AddLI(li)
		if err != nil {
			return "", err
		}
	case "CLI_LI_UPDATE":
		li := &LocalInterceptor{}
		err := json.Unmarshal([]byte(cliCommand.Data), li)
		if err != nil {
			return "", err
		}
		err = masterData.UpdateLI(li)
		if err != nil {
			return "", err
		}
	case "CLI_LI_REMOVE":
		li := &LocalInterceptor{}
		err := json.Unmarshal([]byte(cliCommand.Data), li)
		if err != nil {
			return "", err
		}
		err = masterData.RemoveLI(li.Id, li.AppId)
		if err != nil {
			return "", err
		}
	case "CLI_RI_ADD":
		ri := &RemoteInterceptor{}
		err := json.Unmarshal([]byte(cliCommand.Data), ri)
		if err != nil {
			return "", err
		}
		err = masterData.AddRI(ri)
		if err != nil {
			return "", err
		}
	case "CLI_RI_UPDATE":
		ri := &RemoteInterceptor{}
		err := json.Unmarshal([]byte(cliCommand.Data), ri)
		if err != nil {
			return "", err
		}
		err = masterData.UpdateRI(ri)
		if err != nil {
			return "", err
		}
	case "CLI_RI_REMOVE":
		ri := &RemoteInterceptor{}
		err := json.Unmarshal([]byte(cliCommand.Data), ri)
		if err != nil {
			return "", err
		}
		err = masterData.RemoveRI(ri.Id, ri.AppId)
		if err != nil {
			return "", err
		}
	case "CLI_SHOW_MASTER":
		masterDataBytes, err := json.Marshal(masterData)
		if err != nil {
			return "", err
		}
		return string(masterDataBytes), nil
	case "CLI_SHOW_API_NODES":
		apiNodesBytes, err := json.Marshal(apiNodes)
		if err != nil {
			return "", err
		}
		return string(apiNodesBytes), nil
	case "CLI_PROPAGATE":
		err := masterData.Propagate()
		if err != nil {
			return "", err
		}
		return "", nil
	}
	return "", nil
}
