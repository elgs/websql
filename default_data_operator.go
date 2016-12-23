// default_data_operator
package websql

import (
	"database/sql"
)

type DefaultDataOperator struct {
}

func (this *DefaultDataOperator) Load(resourceId string, id string, fields string, context map[string]interface{}) (map[string]string, error) {
	return nil, nil
}
func (this *DefaultDataOperator) ListMap(resourceId string, fields string, filter []string, sort string, group string, start int64, limit int64, context map[string]interface{}) ([]map[string]string, int64, error) {
	return nil, -1, nil
}
func (this *DefaultDataOperator) ListArray(resourceId string, fields string, filter []string, sort string, group string, start int64, limit int64, context map[string]interface{}) ([]string, [][]string, int64, error) {
	return nil, nil, -1, nil
}
func (this *DefaultDataOperator) Create(resourceId string, data map[string]interface{}, context map[string]interface{}) (interface{}, error) {
	return nil, nil
}
func (this *DefaultDataOperator) Update(resourceId string, data map[string]interface{}, context map[string]interface{}) (int64, error) {
	return -1, nil
}
func (this *DefaultDataOperator) Duplicate(resourceId string, id string, context map[string]interface{}) (interface{}, error) {
	return nil, nil
}
func (this *DefaultDataOperator) Delete(resourceId string, id string, context map[string]interface{}) (int64, error) {
	return -1, nil
}
func (this *DefaultDataOperator) MetaData(resourceId string) ([]map[string]string, error) {
	return nil, nil
}

func (this *DefaultDataOperator) Exec(resourceId string, params [][]interface{}, queryParams map[string]string, array bool, context map[string]interface{}) ([][]interface{}, error) {
	return nil, nil
}

func (this *DefaultDataOperator) GetConn() (*sql.DB, error) {
	return nil, nil
}
