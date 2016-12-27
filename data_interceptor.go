package websql

import (
	"database/sql"
	"net/http"
	"sort"
	"strings"
)

type Interceptors struct {
	GlobalDataInterceptorRegistry    map[int]DataInterceptor
	DataInterceptorRegistry          map[string]map[int]DataInterceptor
	GlobalHandlerInterceptorRegistry []HandlerInterceptor
	HandlerInterceptorRegistry       map[string]HandlerInterceptor
}

type DefaultDataInterceptor struct{}
type DefaultHandlerInterceptor struct{}

//var GlobalDataInterceptorRegistry = map[int]DataInterceptor{}
//var DataInterceptorRegistry = map[string]map[int]DataInterceptor{}
//var GlobalHandlerInterceptorRegistry = []HandlerInterceptor{}
//var HandlerInterceptorRegistry = map[string]HandlerInterceptor{}

func (this *Interceptors) RegisterDataInterceptor(id string, seq int, dataInterceptor DataInterceptor) {
	id = strings.Replace(strings.ToUpper(id), "`", "", -1)
	if this.DataInterceptorRegistry[id] == nil {
		this.DataInterceptorRegistry[id] = make(map[int]DataInterceptor)
	}
	this.DataInterceptorRegistry[id][seq] = dataInterceptor
}

func (this *Interceptors) GetDataInterceptors(id string) (map[int]DataInterceptor, []int) {
	interceptors := this.DataInterceptorRegistry[strings.ToUpper(strings.Replace(id, "`", "", -1))]
	keys := make([]int, 0)
	for k := range interceptors {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return interceptors, keys
}

func (this *Interceptors) RegisterGlobalDataInterceptor(seq int, globalDataInterceptor DataInterceptor) {
	this.GlobalDataInterceptorRegistry[seq] = globalDataInterceptor
}

func (this *Interceptors) GetGlobalDataInterceptors() (map[int]DataInterceptor, []int) {
	keys := make([]int, 0)
	for k := range this.GlobalDataInterceptorRegistry {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return this.GlobalDataInterceptorRegistry, keys
}

type DataInterceptor interface {
	BeforeLoad(resourceId string, db *sql.DB, fields string, context map[string]interface{}, id string) error
	AfterLoad(resourceId string, db *sql.DB, fields string, context map[string]interface{}, data map[string]string) error
	BeforeCreate(resourceId string, db *sql.DB, context map[string]interface{}, data []map[string]interface{}) error
	AfterCreate(resourceId string, db *sql.DB, context map[string]interface{}, data []map[string]interface{}) error
	BeforeUpdate(resourceId string, db *sql.DB, context map[string]interface{}, data []map[string]interface{}) error
	AfterUpdate(resourceId string, db *sql.DB, context map[string]interface{}, data []map[string]interface{}) error
	BeforeDuplicate(resourceId string, db *sql.DB, context map[string]interface{}, id []string) error
	AfterDuplicate(resourceId string, db *sql.DB, context map[string]interface{}, oldId []string, newId []string) error
	BeforeDelete(resourceId string, db *sql.DB, context map[string]interface{}, id []string) error
	AfterDelete(resourceId string, db *sql.DB, context map[string]interface{}, id []string) error
	BeforeListMap(resourceId string, db *sql.DB, fields string, context map[string]interface{}, filter *string, sort *string, group *string, start int64, limit int64) error
	AfterListMap(resourceId string, db *sql.DB, fields string, context map[string]interface{}, data *[]map[string]string, total int64) error
	BeforeListArray(resourceId string, db *sql.DB, fields string, context map[string]interface{}, filter *string, sort *string, group *string, start int64, limit int64) error
	AfterListArray(resourceId string, db *sql.DB, fields string, context map[string]interface{}, headers *[]string, data *[][]string, total int64) error
	BeforeExec(resourceId string, script string, params *[][]interface{}, queryParams map[string]string, array bool, db *sql.DB, context map[string]interface{}) error
	AfterExec(resourceId string, script string, params *[][]interface{}, queryParams map[string]string, array bool, db *sql.DB, context map[string]interface{}, data *[][]interface{}) error
}

type HandlerInterceptor interface {
	BeforeHandle(w http.ResponseWriter, r *http.Request) error
	AfterHandle(w http.ResponseWriter, r *http.Request) error
}

func (this *DefaultDataInterceptor) BeforeLoad(resourceId string, db *sql.DB, fields string, context map[string]interface{}, id string) error {
	return nil
}
func (this *DefaultDataInterceptor) AfterLoad(resourceId string, db *sql.DB, fields string, context map[string]interface{}, data map[string]string) error {
	return nil
}
func (this *DefaultDataInterceptor) BeforeCreate(resourceId string, db *sql.DB, context map[string]interface{}, data []map[string]interface{}) error {
	return nil
}
func (this *DefaultDataInterceptor) AfterCreate(resourceId string, db *sql.DB, context map[string]interface{}, data []map[string]interface{}) error {
	return nil
}
func (this *DefaultDataInterceptor) BeforeUpdate(resourceId string, db *sql.DB, context map[string]interface{}, data []map[string]interface{}) error {
	return nil
}
func (this *DefaultDataInterceptor) AfterUpdate(resourceId string, db *sql.DB, context map[string]interface{}, data []map[string]interface{}) error {
	return nil
}
func (this *DefaultDataInterceptor) BeforeDuplicate(resourceId string, db *sql.DB, context map[string]interface{}, id []string) error {
	return nil
}
func (this *DefaultDataInterceptor) AfterDuplicate(resourceId string, db *sql.DB, context map[string]interface{}, oldId []string, newId []string) error {
	return nil
}
func (this *DefaultDataInterceptor) BeforeDelete(resourceId string, db *sql.DB, context map[string]interface{}, id []string) error {
	return nil
}
func (this *DefaultDataInterceptor) AfterDelete(resourceId string, db *sql.DB, context map[string]interface{}, id []string) error {
	return nil
}
func (this *DefaultDataInterceptor) BeforeListMap(resourceId string, db *sql.DB, fields string, context map[string]interface{}, filter *string, sort *string, group *string, start int64, limit int64) error {
	return nil
}
func (this *DefaultDataInterceptor) AfterListMap(resourceId string, db *sql.DB, fields string, context map[string]interface{}, data *[]map[string]string, total int64) error {
	return nil
}
func (this *DefaultDataInterceptor) BeforeListArray(resourceId string, db *sql.DB, fields string, context map[string]interface{}, filter *string, sort *string, group *string, start int64, limit int64) error {
	return nil
}
func (this *DefaultDataInterceptor) AfterListArray(resourceId string, db *sql.DB, fields string, context map[string]interface{}, headers *[]string, data *[][]string, total int64) error {
	return nil
}

func (this *DefaultDataInterceptor) BeforeExec(resourceId string, script string, params *[][]interface{}, queryParams map[string]string, array bool, db *sql.DB, context map[string]interface{}) error {
	return nil
}
func (this *DefaultDataInterceptor) AfterExec(resourceId string, script string, params *[][]interface{}, queryParams map[string]string, array bool, db *sql.DB, context map[string]interface{}, data *[][]interface{}) error {
	return nil
}

func (this *DefaultHandlerInterceptor) BeforeHandle(w http.ResponseWriter, r *http.Request) error {
	return nil
}
func (this *DefaultHandlerInterceptor) AfterHandle(w http.ResponseWriter, r *http.Request) error {
	return nil
}
