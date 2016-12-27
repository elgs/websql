// login_interceptor
package websql

import (
	"database/sql"
)

func init() {
	tableId := "forget_password"
	Websql.interceptors.RegisterDataInterceptor(tableId, 0, &ForgetPasswordInterceptor{Id: tableId})
}

type ForgetPasswordInterceptor struct {
	*DefaultDataInterceptor
	Id string
}

func (this *ForgetPasswordInterceptor) AfterExec(resourceId string, script string, params *[][]interface{}, queryParams map[string]string, array bool, db *sql.DB, context map[string]interface{}, data *[][]interface{}) error {
	userInfo := (*data)[0][3]
	if userMap, ok := userInfo.([]map[string]string); ok {
		if len(userMap) > 0 {
			SendMail("UpRun User Verification", userMap[0]["VERIFICATION_CODE"], userMap[0]["EMAIL"])
		}
	}
	(*data)[0][3] = ""
	return nil
}
