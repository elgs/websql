// login_interceptor
package websql

import (
	"database/sql"
)

func init() {
	tableId := "signup"
	RegisterDataInterceptor(tableId, 0, &SginupInterceptor{Id: tableId})
}

type SginupInterceptor struct {
	*DefaultDataInterceptor
	Id string
}

func (this *SginupInterceptor) AfterExec(resourceId string, script string, params *[][]interface{}, queryParams map[string]string, array bool, db *sql.DB, context map[string]interface{}, data *[][]interface{}) error {
	userInfo := (*data)[0][5]
	if userMap, ok := userInfo.([]map[string]string); ok {
		SendMail("UpRun User Verification", userMap[0]["VERIFICATION_CODE"], userMap[0]["EMAIL"])
	}
	(*data)[0][5] = ""
	return nil
}
