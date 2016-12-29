// login_interceptor
package websql

import (
	"database/sql"
	"encoding/json"
	"time"
)

func init() {
	tableId := "login"
	Websql.Interceptors.RegisterDataInterceptor(tableId, 0, &LoginInterceptor{Id: tableId})
}

type LoginInterceptor struct {
	*DefaultDataInterceptor
	Id string
}

func (this *LoginInterceptor) AfterExec(resourceId string, script string, params *[][]interface{}, queryParams map[string]string, array bool, db *sql.DB, context map[string]interface{}, data *[][]interface{}) error {
	// if the query name is login, encrypt the query result into a jwt token.
	tokenData := (*data)[0][0]
	if v, ok := tokenData.([]map[string]string); ok && len(v) > 0 {
		t, err := convertMapOfStringsToMapOfInterfaces(v[0])
		if err != nil {
			return err
		}
		t["exp"] = time.Now().Add(time.Hour * 72).Unix()
		tokenPayload, err := json.Marshal(t)
		if err != nil {
			return err
		}
		s, err := createJwtToken(string(tokenPayload))
		if err != nil {
			return err
		}
		(*data)[0][0] = s
	} else {
		(*data)[0][0] = ""
	}
	return nil
}
