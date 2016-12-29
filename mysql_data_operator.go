package websql

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/elgs/exparser"
	"github.com/elgs/gosqljson"
	"github.com/satori/go.uuid"
)

type MySqlDataOperator struct {
	*DefaultDataOperator
	Ds     string
	DbType string
	db     *sql.DB
}

func (this *MySqlDataOperator) Load(tableId string, id string, fields string, context map[string]interface{}) (map[string]string, error) {
	ret := make(map[string]string, 0)
	tableId = normalizeTableId(tableId, this.DbType, this.Ds)
	db, err := this.GetConn()

	globalDataInterceptors, globalSortedKeys := Websql.Interceptors.GetGlobalDataInterceptors()
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.BeforeLoad(tableId, db, fields, context, id)
		if err != nil {
			return ret, err
		}
	}
	dataInterceptors, sortedKeys := Websql.Interceptors.GetDataInterceptors(tableId)
	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.BeforeLoad(tableId, db, fields, context, id)
			if err != nil {
				return ret, err
			}
		}
	}

	// Load the record
	extraFilter := context["extra_filter"]
	if extraFilter == nil {
		extraFilter = ""
	}
	c := context["case"].(string)

	m, err := gosqljson.QueryDbToMap(db, c,
		fmt.Sprint("SELECT ", fields, " FROM ", tableId, " WHERE ID=? ", extraFilter), id)
	if err != nil {
		fmt.Println(err)
		return ret, err
	}

	if len(m) == 0 {
		m = []map[string]string{
			make(map[string]string, 0),
		}
	}

	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			dataInterceptor.AfterLoad(tableId, db, fields, context, m[0])
		}
	}
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		globalDataInterceptor.AfterLoad(tableId, db, fields, context, m[0])
	}

	if m != nil && len(m) == 1 {
		return m[0], err
	} else {
		return ret, err
	}
}

func (this *MySqlDataOperator) ListMap(tableId string, fields string, filter []string, sort string, group string,
	start int64, limit int64, context map[string]interface{}) ([]map[string]string, int64, error) {
	ret := make([]map[string]string, 0)
	tableId = normalizeTableId(tableId, this.DbType, this.Ds)
	db, err := this.GetConn()
	if err != nil {
		return nil, -1, err
	}
	tx, err := db.Begin()
	if err != nil {
		tx.Rollback()
		return nil, -1, err
	}

	sort = parseSort(sort)
	where := parseFilters(filter)
	//	fmt.Println(where)
	globalDataInterceptors, globalSortedKeys := Websql.Interceptors.GetGlobalDataInterceptors()
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.BeforeListMap(tableId, db, fields, context, &where, &sort, &group, start, limit)
		if err != nil {
			return ret, -1, err
		}
	}
	dataInterceptors, sortedKeys := Websql.Interceptors.GetDataInterceptors(tableId)
	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.BeforeListMap(tableId, db, fields, context, &where, &sort, &group, start, limit)
			if err != nil {
				return ret, -1, err
			}
		}
	}
	c := context["case"].(string)
	sqlQuery := fmt.Sprint("SELECT SQL_CALC_FOUND_ROWS ", fields, " FROM ", tableId, where, parseGroup(group), sort, " LIMIT ?,?")
	cnt := -1
	m, err := gosqljson.QueryTxToMap(tx, c, sqlQuery, start, limit)
	if err != nil {
		tx.Rollback()
		fmt.Println(err)
		return nil, -1, err
	}

	cntData, err := gosqljson.QueryTxToMap(tx, "upper",
		fmt.Sprint("SELECT FOUND_ROWS()"))
	if err != nil {
		tx.Rollback()
		return nil, -1, err
	}
	cnt, err = strconv.Atoi(cntData[0]["FOUND_ROWS()"])
	if err != nil {
		tx.Rollback()
		return nil, -1, err
	}

	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			dataInterceptor.AfterListMap(tableId, db, fields, context, &m, int64(cnt))
		}
	}
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		globalDataInterceptor.AfterListMap(tableId, db, fields, context, &m, int64(cnt))
	}
	tx.Commit()

	return m, int64(cnt), err
}
func (this *MySqlDataOperator) ListArray(tableId string, fields string, filter []string, sort string, group string,
	start int64, limit int64, context map[string]interface{}) ([]string, [][]string, int64, error) {
	tableId = normalizeTableId(tableId, this.DbType, this.Ds)
	db, err := this.GetConn()
	if err != nil {
		return nil, nil, -1, err
	}
	tx, err := db.Begin()
	if err != nil {
		tx.Rollback()
		return nil, nil, -1, err
	}

	sort = parseSort(sort)
	where := parseFilters(filter)
	globalDataInterceptors, globalSortedKeys := Websql.Interceptors.GetGlobalDataInterceptors()
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.BeforeListArray(tableId, db, fields, context, &where, &sort, &group, start, limit)
		if err != nil {
			return nil, nil, -1, err
		}
	}
	dataInterceptors, sortedKeys := Websql.Interceptors.GetDataInterceptors(tableId)
	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.BeforeListArray(tableId, db, fields, context, &where, &sort, &group, start, limit)
			if err != nil {
				return nil, nil, -1, err
			}
		}
	}

	c := context["case"].(string)
	h, a, err := gosqljson.QueryTxToArray(tx, c,
		fmt.Sprint("SELECT SQL_CALC_FOUND_ROWS ", fields, " FROM ", tableId, where, parseGroup(group), sort, " LIMIT ?,?"), start, limit)
	if err != nil {
		tx.Rollback()
		return nil, nil, -1, err
	}
	cnt := -1
	cntData, err := gosqljson.QueryTxToMap(tx, "upper",
		fmt.Sprint("SELECT FOUND_ROWS()"))
	if err != nil {
		tx.Rollback()
		return nil, nil, -1, err
	}
	cnt, err = strconv.Atoi(cntData[0]["FOUND_ROWS()"])
	if err != nil {
		tx.Rollback()
		return nil, nil, -1, err
	}

	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			dataInterceptor.AfterListArray(tableId, db, fields, context, &h, &a, int64(cnt))
		}
	}
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		globalDataInterceptor.AfterListArray(tableId, db, fields, context, &h, &a, int64(cnt))
	}
	tx.Commit()

	return h, a, int64(cnt), err
}

func (this *MySqlDataOperator) Create(tableId string, data []map[string]interface{}, context map[string]interface{}) ([]interface{}, error) {
	tableId = normalizeTableId(tableId, this.DbType, this.Ds)
	db, err := this.GetConn()

	globalDataInterceptors, globalSortedKeys := Websql.Interceptors.GetGlobalDataInterceptors()
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.BeforeCreate(tableId, db, context, data)
		if err != nil {
			if tx, ok := context["tx"].(*sql.Tx); ok {
				tx.Rollback()
			}
			return nil, err
		}
	}
	dataInterceptors, sortedKeys := Websql.Interceptors.GetDataInterceptors(tableId)
	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.BeforeCreate(tableId, db, context, data)
			if err != nil {
				if tx, ok := context["tx"].(*sql.Tx); ok {
					tx.Rollback()
				}
				return nil, err
			}
		}
	}
	// Create the record
	ret := []interface{}{}
	for _, data1 := range data {
		if data1["ID"] == nil || data1["ID"].(string) == "" {
			data1["ID"] = strings.Replace(uuid.NewV4().String(), "-", "", -1)
		}
		ret = append(ret, data1["ID"])
		dataLen := len(data1)
		values := make([]interface{}, 0, dataLen)
		var fieldBuffer bytes.Buffer
		var qmBuffer bytes.Buffer
		count := 0
		for k, v := range data1 {
			count++
			if count == dataLen {
				fieldBuffer.WriteString(k)
				qmBuffer.WriteString("?")
			} else {
				fieldBuffer.WriteString(fmt.Sprint(k, ","))
				qmBuffer.WriteString("?,")
			}
			values = append(values, v)
		}
		fields := fieldBuffer.String()
		qms := qmBuffer.String()
		if tx, ok := context["tx"].(*sql.Tx); ok {
			_, err = gosqljson.ExecTx(tx, fmt.Sprint("INSERT INTO ", tableId, " (", fields, ") VALUES (", qms, ")"), values...)
			if err != nil {
				fmt.Println(err)
				tx.Rollback()
				return nil, err
			}
		} else {
			_, err = gosqljson.ExecDb(db, fmt.Sprint("INSERT INTO ", tableId, " (", fields, ") VALUES (", qms, ")"), values...)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
		}
	}

	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.AfterCreate(tableId, db, context, data)
			if err != nil {
				if tx, ok := context["tx"].(*sql.Tx); ok {
					tx.Rollback()
				}
				return nil, err
			}
		}
	}
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.AfterCreate(tableId, db, context, data)
		if err != nil {
			if tx, ok := context["tx"].(*sql.Tx); ok {
				tx.Rollback()
			}
			return nil, err
		}
	}

	if tx, ok := context["tx"].(*sql.Tx); ok {
		tx.Commit()
	}

	return ret, err
}
func (this *MySqlDataOperator) Update(tableId string, data []map[string]interface{}, context map[string]interface{}) ([]int64, error) {
	tableId = normalizeTableId(tableId, this.DbType, this.Ds)
	db, err := this.GetConn()

	globalDataInterceptors, globalSortedKeys := Websql.Interceptors.GetGlobalDataInterceptors()
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.BeforeUpdate(tableId, db, context, data)
		if err != nil {
			if tx, ok := context["tx"].(*sql.Tx); ok {
				tx.Rollback()
			}
			return nil, err
		}
	}
	dataInterceptors, sortedKeys := Websql.Interceptors.GetDataInterceptors(tableId)
	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.BeforeUpdate(tableId, db, context, data)
			if err != nil {
				if tx, ok := context["tx"].(*sql.Tx); ok {
					tx.Rollback()
				}
				return nil, err
			}
		}
	}
	ret := []int64{}
	// Update the record
	for _, data1 := range data {
		id := data1["ID"]
		if id == nil {
			fmt.Println("ID is not found.")
			if tx, ok := context["tx"].(*sql.Tx); ok {
				tx.Rollback()
			}
			return nil, err
		}
		delete(data1, "ID")
		dataLen := len(data1)
		values := make([]interface{}, 0, dataLen)
		var buffer bytes.Buffer
		for k, v := range data1 {
			buffer.WriteString(fmt.Sprint(k, "=?,"))
			values = append(values, v)
		}
		values = append(values, id)
		sets := buffer.String()
		sets = sets[0 : len(sets)-1]
		var rowsAffected int64 = 0
		if tx, ok := context["tx"].(*sql.Tx); ok {
			load, _ := context["load"].(bool)
			if load {
				data, err := gosqljson.QueryTxToMap(tx, "upper", "SELECT * FROM "+tableId+" WHERE ID=?", id)
				if err != nil {
					fmt.Println(err)
					tx.Rollback()
					return nil, err
				}
				if data == nil && len(data) != 1 {
					tx.Rollback()
					return nil, errors.New(id.(string) + " not found.")
				} else {
					context["old_data"] = data[0]
				}
			}

			rowsAffected, err = gosqljson.ExecTx(tx, fmt.Sprint("UPDATE ", tableId, " SET ", sets, " WHERE ID=?"), values...)
			if err != nil {
				fmt.Println(err)
				tx.Rollback()
				return nil, err
			}
		} else {
			load, _ := context["load"].(bool)
			if load {
				data, err := gosqljson.QueryDbToMap(db, "upper", "SELECT * FROM "+tableId+" WHERE ID=?", id)
				if err != nil {
					fmt.Println(err)
					return nil, err
				}
				if data == nil && len(data) != 1 {
					return nil, errors.New(id.(string) + " not found.")
				} else {
					context["old_data"] = data[0]
				}
			}

			rowsAffected, err = gosqljson.ExecDb(db, fmt.Sprint("UPDATE ", tableId, " SET ", sets, " WHERE ID=?"), values...)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
		}
		data1["ID"] = id
		ret = append(ret, rowsAffected)
	}

	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.AfterUpdate(tableId, db, context, data)
			if err != nil {
				if tx, ok := context["tx"].(*sql.Tx); ok {
					tx.Rollback()
				}
				return nil, err
			}
		}
	}
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.AfterUpdate(tableId, db, context, data)
		if err != nil {
			if tx, ok := context["tx"].(*sql.Tx); ok {
				tx.Rollback()
			}
			return nil, err
		}
	}

	if tx, ok := context["tx"].(*sql.Tx); ok {
		tx.Commit()
	}

	return ret, err
}
func (this *MySqlDataOperator) Duplicate(tableId string, id []string, context map[string]interface{}) ([]string, error) {
	tableId = normalizeTableId(tableId, this.DbType, this.Ds)
	db, err := this.GetConn()

	globalDataInterceptors, globalSortedKeys := Websql.Interceptors.GetGlobalDataInterceptors()
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.BeforeDuplicate(tableId, db, context, id)
		if err != nil {
			if tx, ok := context["tx"].(*sql.Tx); ok {
				tx.Rollback()
			}
			return nil, err
		}
	}
	dataInterceptors, sortedKeys := Websql.Interceptors.GetDataInterceptors(tableId)
	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.BeforeDuplicate(tableId, db, context, id)
			if err != nil {
				if tx, ok := context["tx"].(*sql.Tx); ok {
					tx.Rollback()
				}
				return nil, err
			}
		}
	}

	ret := []string{}
	for _, id1 := range id {
		newId := strings.Replace(uuid.NewV4().String(), "-", "", -1)
		// Duplicate the record
		if tx, ok := context["tx"].(*sql.Tx); ok {
			data, err := gosqljson.QueryTxToMap(tx, "upper",
				fmt.Sprint("SELECT * FROM ", tableId, " WHERE ID=?"), id1)
			if data == nil || len(data) != 1 {
				tx.Rollback()
				return nil, err
			}
			newData := make(map[string]interface{}, len(data[0]))
			for k, v := range data[0] {
				newData[k] = v
			}
			newData["ID"] = newId

			newDataLen := len(newData)
			newValues := make([]interface{}, 0, newDataLen)
			var fieldBuffer bytes.Buffer
			var qmBuffer bytes.Buffer
			count := 0
			for k, v := range newData {
				count++
				if count == newDataLen {
					fieldBuffer.WriteString(k)
					qmBuffer.WriteString("?")
				} else {
					fieldBuffer.WriteString(fmt.Sprint(k, ","))
					qmBuffer.WriteString("?,")
				}
				newValues = append(newValues, v)
			}
			fields := fieldBuffer.String()
			qms := qmBuffer.String()
			_, err = gosqljson.ExecTx(tx, fmt.Sprint("INSERT INTO ", tableId, " (", fields, ") VALUES (", qms, ")"), newValues...)
			if err != nil {
				fmt.Println(err)
				tx.Rollback()
				return nil, err
			}
		} else {
			data, err := gosqljson.QueryDbToMap(db, "upper",
				fmt.Sprint("SELECT * FROM ", tableId, " WHERE ID=?"), id1)
			if data == nil || len(data) != 1 {
				return nil, err
			}
			newData := make(map[string]interface{}, len(data[0]))
			for k, v := range data[0] {
				newData[k] = v
			}
			newData["ID"] = newId

			newDataLen := len(newData)
			newValues := make([]interface{}, 0, newDataLen)
			var fieldBuffer bytes.Buffer
			var qmBuffer bytes.Buffer
			count := 0
			for k, v := range newData {
				count++
				if count == newDataLen {
					fieldBuffer.WriteString(k)
					qmBuffer.WriteString("?")
				} else {
					fieldBuffer.WriteString(fmt.Sprint(k, ","))
					qmBuffer.WriteString("?,")
				}
				newValues = append(newValues, v)
			}
			fields := fieldBuffer.String()
			qms := qmBuffer.String()
			_, err = gosqljson.ExecDb(db, fmt.Sprint("INSERT INTO ", tableId, " (", fields, ") VALUES (", qms, ")"), newValues...)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
		}
		ret = append(ret, newId)
	}

	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.AfterDuplicate(tableId, db, context, id, ret)
			if err != nil {
				if tx, ok := context["tx"].(*sql.Tx); ok {
					tx.Rollback()
				}
				return nil, err
			}
		}
	}
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.AfterDuplicate(tableId, db, context, id, ret)
		if err != nil {
			if tx, ok := context["tx"].(*sql.Tx); ok {
				tx.Rollback()
			}
			return nil, err
		}
	}

	if tx, ok := context["tx"].(*sql.Tx); ok {
		tx.Commit()
	}

	return ret, err
}

func (this *MySqlDataOperator) Delete(tableId string, id []string, context map[string]interface{}) ([]int64, error) {
	tableId = normalizeTableId(tableId, this.DbType, this.Ds)
	db, err := this.GetConn()

	globalDataInterceptors, globalSortedKeys := Websql.Interceptors.GetGlobalDataInterceptors()
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.BeforeDelete(tableId, db, context, id)
		if err != nil {
			if tx, ok := context["tx"].(*sql.Tx); ok {
				tx.Rollback()
			}
			return nil, err
		}
	}
	dataInterceptors, sortedKeys := Websql.Interceptors.GetDataInterceptors(tableId)
	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.BeforeDelete(tableId, db, context, id)
			if err != nil {
				if tx, ok := context["tx"].(*sql.Tx); ok {
					tx.Rollback()
				}
				return nil, err
			}
		}
	}

	ret := []int64{}
	for _, id1 := range id {
		var rowsAffected int64 = 0
		if tx, ok := context["tx"].(*sql.Tx); ok {
			load, _ := context["load"].(bool)
			if load {
				data, err := gosqljson.QueryTxToMap(tx, "upper", "SELECT * FROM "+tableId+" WHERE ID=?", id1)
				if err != nil {
					fmt.Println(err)
					tx.Rollback()
					return nil, err
				}
				if data == nil && len(data) != 1 {
					tx.Rollback()
					return nil, errors.New(id1 + " not found.")
				} else {
					context["old_data"] = data[0]
				}
			}

			// Delete the record
			rowsAffected, err = gosqljson.ExecTx(tx, fmt.Sprint("DELETE FROM ", tableId, " WHERE ID=?"), id1)
			if err != nil {
				fmt.Println(err)
				tx.Rollback()
				return nil, err
			}
		} else {
			load, _ := context["load"].(bool)
			if load {
				data, err := gosqljson.QueryDbToMap(db, "upper", "SELECT * FROM "+tableId+" WHERE ID=?", id1)
				if err != nil {
					fmt.Println(err)
					return nil, err
				}
				if data == nil && len(data) != 1 {
					return nil, errors.New(id1 + " not found.")
				} else {
					context["old_data"] = data[0]
				}
			}

			// Delete the record
			rowsAffected, err = gosqljson.ExecDb(db, fmt.Sprint("DELETE FROM ", tableId, " WHERE ID=?"), id1)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
		}
		ret = append(ret, rowsAffected)
	}

	for _, k := range sortedKeys {
		dataInterceptor := dataInterceptors[k]
		if dataInterceptor != nil {
			err := dataInterceptor.AfterDelete(tableId, db, context, id)
			if err != nil {
				if tx, ok := context["tx"].(*sql.Tx); ok {
					tx.Rollback()
				}
				return nil, err
			}
		}
	}
	for _, k := range globalSortedKeys {
		globalDataInterceptor := globalDataInterceptors[k]
		err := globalDataInterceptor.AfterDelete(tableId, db, context, id)
		if err != nil {
			if tx, ok := context["tx"].(*sql.Tx); ok {
				tx.Rollback()
			}
			return nil, err
		}
	}
	if tx, ok := context["tx"].(*sql.Tx); ok {
		tx.Commit()
	}
	return ret, err
}

func (this *MySqlDataOperator) GetConn() (*sql.DB, error) {
	if this.db == nil {
		if len(strings.TrimSpace(this.DbType)) == 0 {
			this.DbType = "mysql"
		}
		db, err := sql.Open(this.DbType, this.Ds)
		//fmt.Println("New db conn created.")
		if err != nil {
			return nil, err
		}
		this.db = db
	}
	return this.db, nil
}

func extractDbNameFromDs(dbType string, ds string) string {
	switch dbType {
	case "sqlite3":
		return ""
	default:
		a := strings.LastIndex(ds, "/")
		b := ds[a+1:]
		c := strings.Index(b, "?")
		if c < 0 {
			return b
		}
		return b[:c]
	}
}

func normalizeTableId(tableId string, dbType string, ds string) string {
	if strings.Contains(tableId, ".") {
		a := strings.Split(tableId, ".")
		return fmt.Sprint("`"+a[0], "`.`", a[1]+"`")
	}
	db := extractDbNameFromDs(dbType, ds)

	MysqlSafe(&tableId)
	if len(strings.TrimSpace(db)) == 0 {
		return "`" + tableId + "`"
	} else {
		MysqlSafe(&db)
		return fmt.Sprint("`"+db+"`", ".", "`"+tableId+"`")
	}
}

func MysqlSafe(s *string) {
	*s = strings.Replace(*s, "'", "''", -1)
	*s = strings.Replace(*s, "--", "", -1)
}

func parseSort(sort string) string {
	if len(strings.TrimSpace(sort)) == 0 {
		return ""
	}
	return fmt.Sprint(" ORDER BY ", strings.ToUpper(strings.Replace(sort, ":", " ", -1)), " ")
}

func parseFilter(filter string) string {
	r, err := parser.Calculate(filter)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return r
}

func parseFilters(filters []string) (r string) {
	for _, v := range filters {
		r += fmt.Sprint("AND ", parseFilter(v))
	}
	r = fmt.Sprint(" WHERE 1=1 ", r, " ")
	//fmt.Println(r)
	return
}

var parser = &exparser.Parser{
	Operators: exparser.MysqlOperators,
}

func parseGroup(group string) (r string) {
	if strings.TrimSpace(group) == "" {
		return ""
	}
	r = fmt.Sprint(" GROUP BY ", strings.ToUpper(group))
	return
}
