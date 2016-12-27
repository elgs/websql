// rest
package websql

import (
	"fmt"
	"net/http"
	"strings"
)

func serve(service *CliService) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", r.Header.Get("Access-Control-Request-Method"))
		w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))

		if r.Method == "OPTIONS" {
			return
		}

		urlPath := r.URL.Path
		var dataHandler func(w http.ResponseWriter, r *http.Request)
		if strings.HasPrefix(urlPath, "/api/") {
			dataHandler = Websql.handlers.GetHandler("/api")
		} else {
			dataHandler = Websql.handlers.GetHandler(urlPath)
		}
		if dataHandler == nil {
			http.Error(w, "Not found.", http.StatusNotFound)
			return
		}
		for _, globalHandlerInterceptor := range Websql.interceptors.GlobalHandlerInterceptorRegistry {
			err := globalHandlerInterceptor.BeforeHandle(w, r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		handlerInterceptor := Websql.interceptors.HandlerInterceptorRegistry[urlPath]
		if handlerInterceptor != nil {
			err := handlerInterceptor.BeforeHandle(w, r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		dataHandler(w, r)
		if handlerInterceptor != nil {
			err := handlerInterceptor.AfterHandle(w, r)
			if err != nil {
				fmt.Fprint(w, err.Error())
				return
			}
		}
		for _, globalHandlerInterceptor := range Websql.interceptors.GlobalHandlerInterceptorRegistry {
			err := globalHandlerInterceptor.AfterHandle(w, r)
			if err != nil {
				fmt.Fprint(w, err.Error())
				return
			}
		}
	}

	http.HandleFunc("/", handler)

	if service.EnableHttp {
		go func() {
			fmt.Println(fmt.Sprint("Listening on http://", service.HttpHost, ":", service.HttpPort, "/"))
			err := http.ListenAndServe(fmt.Sprint(service.HttpHost, ":", service.HttpPort), nil)
			if err != nil {
				fmt.Println(err)
			}
		}()
	}
	if service.EnableHttps {
		go func() {
			fmt.Println(fmt.Sprint("Listening on https://", service.HttpsHost, ":", service.HttpsPort, "/"))
			err := http.ListenAndServeTLS(fmt.Sprint(service.HttpsHost, ":", service.HttpsPort), service.CertFile, service.KeyFile, nil)
			if err != nil {
				fmt.Println(err)
			}
		}()
	}
}
