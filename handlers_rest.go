// handlers_rest
package websql

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/elgs/gojq"
	"github.com/elgs/gosplitargs"
)

var RequestWrites = map[string]int{}
var RequestReads = map[string]int{}

var translateBoolParam = func(field string, defaultValue bool) bool {
	if field == "1" {
		return true
	} else if field == "0" {
		return false
	} else {
		return defaultValue
	}
}

//var convertMapOfInterfacesToMapOfStrings = func(data map[string]interface{}) (map[string]string, error) {
//	if data == nil {
//		return nil, errors.New("Cannot convert nil.")
//	}
//	ret := map[string]string{}
//	for k, v := range data {
//		if v == nil {
//			return nil, errors.New("Data contains nil.")
//		}
//		ret[k] = v.(string)
//	}
//	return ret, nil
//}

var parseExecParams = func(data string) (retQp map[string]string, retP [][]interface{}, retArray bool, retCase string, err error) {
	retQp = map[string]string{}
	retP = [][]interface{}{}
	retArray = true
	parser, err := gojq.NewStringQuery(data)
	if err != nil {
		return
	}
	qp, errx := parser.Query("query_params")
	if errx == nil {
		switch v := qp.(type) {
		case map[string]interface{}:
			x, errx := convertMapOfInterfacesToMapOfStrings(v)
			if errx != nil {
				err = errx
				return
			}
			retQp = x
		case []interface{}:
			for i, v := range v {
				if v == nil {
					err = errors.New("Data contains nil.")
					return
				}
				retQp[fmt.Sprint("$", i)] = v.(string)
			}
		default:
			err = errors.New("Cannot recognize data type")
			return
		}
	}

	p, errx := parser.Query("params")
	if errx == nil {
		if v, found := p.([]interface{}); found {
			for _, v1 := range v {
				if v1 == nil {
					err = errors.New("Data contains nil.")
					return
				}
				if v2, found := v1.([]interface{}); found {
					// array
					retP = append(retP, v2)
				} else if v2, found := v1.(interface{}); found {
					// item
					if len(retP) == 0 {
						retP = append(retP, []interface{}{})
					}
					retP[0] = append(retP[0], v2)
				}
			}
		} else {
			err = errors.New("Cannot recognize data type")
			return
		}
	}
	if len(retP) == 0 {
		retP = [][]interface{}{[]interface{}{}}
	}

	array, errx := parser.Query("array")
	if errx == nil {
		if v, found := array.(bool); found {
			retArray = v
		} else {
			err = errors.New("Cannot recognize data type")
			return
		}
	}

	theCase, errx := parser.Query("case")
	if errx == nil {
		if v, found := theCase.(string); found {
			retCase = v
		} else {
			err = errors.New("Cannot recognize data type")
			return
		}
	}
	return
}

var RestFunc = func(w http.ResponseWriter, r *http.Request) {
	context := make(map[string]interface{})

	apiToken := r.Header.Get("api-token")
	appId := apiToken[:32]

	context["api_token"] = apiToken

	userToken := r.Header.Get("user-token")
	if len(userToken) > 0 {
		context["user_token"] = userToken
	}

	if appId == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, `{"err":"Invalid app."}`)
		return
	}
	context["app_id"] = appId

	if r.Method == "GET" {
		RequestReads[appId] += 1
	} else {
		RequestWrites[appId] += 1
	}

	dbo, err := GetDbo(appId)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, fmt.Sprintf(`{"err":"%v"}`, err))
		return
	}
	if dbo == nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, `{"err":"Invalid project."}`)
		return
	}

	sepIndex := strings.LastIndex(r.RemoteAddr, ":")
	clientIp := r.RemoteAddr[0:sepIndex]
	context["client_ip"] = strings.Replace(strings.Replace(clientIp, "[", "", -1), "]", "", -1)

	urlPath := r.URL.Path
	urlPathData := strings.Split(urlPath[1:], "/")
	tableId := urlPathData[1]

	switch r.Method {
	case "GET":
		if len(urlPathData) == 2 || len(urlPathData[2]) == 0 {
			//List records.
			fields := strings.ToUpper(r.FormValue("fields"))
			sort := r.FormValue("sort")
			group := r.FormValue("group")
			s := r.FormValue("start")
			l := r.FormValue("limit")
			c := r.FormValue("case")
			p := r.FormValue("params")
			qp := r.FormValue("query_params")
			context["case"] = c
			filter := r.Form["filter"]
			array := translateBoolParam(r.FormValue("array"), false)
			start, err := strconv.ParseInt(s, 10, 0)
			if err != nil {
				start = 0
				err = nil
			}
			limit, err := strconv.ParseInt(l, 10, 0)
			if err != nil {
				limit = 25
				err = nil
			}
			if fields == "" {
				fields = "*"
			}
			params, err := gosplitargs.SplitArgs(p, ",", false)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			parameters := make([]interface{}, len(params))
			for i, v := range params {
				parameters[i] = v
			}

			queryParams, err := gosplitargs.SplitArgs(qp, ",", false)
			_ = queryParams
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			var data interface{}
			var total int64 = -1
			m := map[string]interface{}{}
			if array {
				headers, dataArray, total, err := dbo.ListArray(tableId, fields, filter, sort, group, start, limit, context)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				} else {
					m["headers"] = headers
					m["data"] = dataArray
					m["total"] = total
				}
			} else {
				data, total, err = dbo.ListMap(tableId, fields, filter, sort, group, start, limit, context)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				} else {
					m["data"] = data
					m["total"] = total
				}
			}
			jsonData, err := json.Marshal(m)
			jsonString := string(jsonData)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			fmt.Fprint(w, jsonString)
		} else {
			// Load record by id.
			dataId := urlPathData[2]
			c := r.FormValue("case")
			context["case"] = c

			fields := strings.ToUpper(r.FormValue("fields"))
			if fields == "" {
				fields = "*"
			}

			data, err := dbo.Load(tableId, dataId, fields, context)

			m := map[string]interface{}{
				"data": data,
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			jsonData, _ := json.Marshal(m)
			jsonString := string(jsonData)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			fmt.Fprint(w, jsonString)
		}
	case "POST":
		// Create the record.

		m := map[string]interface{}{}
		var postData interface{}
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&postData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		inputMode := 1
		postDataArray := []interface{}{}
		switch v := postData.(type) {
		case []interface{}:
			inputMode = 2
			postDataArray = v
		case map[string]interface{}:
			postDataArray = append(postDataArray, v)
		default:
			http.Error(w, "Error parsing post data.", http.StatusInternalServerError)
			return
		}

		upperCasePostDataArray := []map[string]interface{}{}
		for _, m := range postDataArray {
			mUpper := map[string]interface{}{}
			if m1, ok := m.(map[string]interface{}); ok {
				for k, v := range m1 {
					if !strings.HasPrefix(k, "_") {
						mUpper[strings.ToUpper(k)] = v
					}
				}
				upperCasePostDataArray = append(upperCasePostDataArray, mUpper)
			}
		}
		data, err := dbo.Create(tableId, upperCasePostDataArray, context)
		if inputMode == 1 && data != nil && len(data) == 1 {
			m["data"] = data[0]
		} else {
			m["data"] = data
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonData, err := json.Marshal(m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonString := string(jsonData)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, jsonString)
	case "PATCH":
		m := map[string]interface{}{}
		result, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		qp, p, array, theCase, err := parseExecParams(string(result))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		context["case"] = theCase
		data, err := dbo.Exec(tableId, p, qp, array, context)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		m["data"] = data
		jsonData, err := json.Marshal(m)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonString := string(jsonData)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, jsonString)
	case "COPY":
		// Duplicate a new record.
		dataIds := []string{}
		if len(urlPathData) >= 3 && len(urlPathData[2]) > 0 {
			dataIds = append(dataIds, urlPathData[2])
		} else {
			var postData interface{}
			decoder := json.NewDecoder(r.Body)
			err := decoder.Decode(&postData)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			postDataArray := []interface{}{}
			switch v := postData.(type) {
			case []interface{}:
				postDataArray = v
			default:
				http.Error(w, "Error parsing post data.", http.StatusInternalServerError)
				return
			}

			for _, postData := range postDataArray {
				if dataId, ok := postData.(string); ok {
					dataIds = append(dataIds, dataId)
				}
			}
		}
		data, err := dbo.Duplicate(tableId, dataIds, context)

		m := map[string]interface{}{}
		if data != nil && len(data) == 1 {
			m["data"] = data[0]
		} else {
			m["data"] = data
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonData, err := json.Marshal(m)
		jsonString := string(jsonData)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, jsonString)
	case "PUT":
		// Update an existing record.

		dataId := ""
		var postData interface{}
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&postData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		inputMode := 1
		postDataArray := []interface{}{}
		switch v := postData.(type) {
		case []interface{}:
			inputMode = 2
			postDataArray = v
		case map[string]interface{}:
			if len(urlPathData) >= 3 && len(urlPathData[2]) > 0 {
				dataId = urlPathData[2]
			}
			postDataArray = append(postDataArray, v)
		default:
			http.Error(w, "Error parsing post data.", http.StatusInternalServerError)
			return
		}

		upperCasePostDataArray := []map[string]interface{}{}
		for _, m := range postDataArray {
			mUpper := map[string]interface{}{}
			if m1, ok := m.(map[string]interface{}); ok {
				for k, v := range m1 {
					if !strings.HasPrefix(k, "_") {
						mUpper[strings.ToUpper(k)] = v
					}
				}
				if inputMode == 1 && dataId != "" {
					mUpper["ID"] = dataId
				}
				upperCasePostDataArray = append(upperCasePostDataArray, mUpper)
			}
		}
		data, err := dbo.Update(tableId, upperCasePostDataArray, context)
		m := map[string]interface{}{}
		if inputMode == 1 && data != nil && len(data) == 1 {
			m["data"] = data[0]
		} else {
			m["data"] = data
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsonData, err := json.Marshal(m)
		jsonString := string(jsonData)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, jsonString)
	case "DELETE":
		// Remove the record.
		dataIds := []string{}
		if len(urlPathData) >= 3 && len(urlPathData[2]) > 0 {
			dataIds = append(dataIds, urlPathData[2])
		} else {
			var postData interface{}
			decoder := json.NewDecoder(r.Body)
			err := decoder.Decode(&postData)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			postDataArray := []interface{}{}
			switch v := postData.(type) {
			case []interface{}:
				postDataArray = v
			default:
				http.Error(w, "Error parsing post data.", http.StatusInternalServerError)
				return
			}

			for _, postData := range postDataArray {
				if dataId, ok := postData.(string); ok {
					dataIds = append(dataIds, dataId)
				}
			}
		}
		data, err := dbo.Delete(tableId, dataIds, context)

		m := map[string]interface{}{}
		if data != nil && len(data) == 1 {
			m["data"] = data[0]
		} else {
			m["data"] = data
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonData, err := json.Marshal(m)
		jsonString := string(jsonData)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, jsonString)
	case "OPTIONS":
	default:
		// Give an error message.
	}
}
