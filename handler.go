// handler
package websql

import (
	"net/http"
)

type Handlers struct {
	handlerRegistry map[string]func(w http.ResponseWriter, r *http.Request)
	DboRegistry     map[string]DataOperator
}

//var handlerRegistry = make(map[string]func(w http.ResponseWriter, r *http.Request))

func (this *Handlers) RegisterHandler(id string, handler func(w http.ResponseWriter, r *http.Request)) {
	Websql.handlers.handlerRegistry[id] = handler
}

func (this *Handlers) GetHandler(id string) func(w http.ResponseWriter, r *http.Request) {
	return Websql.handlers.handlerRegistry[id]
}

//var DboRegistry = make(map[string]DataOperator)

//var GetDbo func(id string) (DataOperator, error)
