// handler
package websql

import (
	"net/http"
)

var handlerRegistry = make(map[string]func(w http.ResponseWriter, r *http.Request))

func RegisterHandler(id string, handler func(w http.ResponseWriter, r *http.Request)) {
	handlerRegistry[id] = handler
}

func GetHandler(id string) func(w http.ResponseWriter, r *http.Request) {
	return handlerRegistry[id]
}

var DboRegistry = make(map[string]DataOperator)

var GetDbo func(id string) (DataOperator, error)
