package mysql

import (
	"database/sql"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var Debug bool

func init() {
	Debug = false // default to false
}

// Result holds the results of a query as map[string]interface{}
type Result map[string]interface{}

type Query struct {

	// Database - database name and primary key, set with New()
	tablename  string
	primarykey string

	// SQL - Private fields used to store sql before building sql query
	sql    string
	sel    []string
	update   string
	join   string
	where  string
	group  string
	having string
	order  string
	offset string
	limit  string

	// Extra args to be substituted in the *where* clause
	args []interface{}
}

func SetData(data map[string]interface{}, object interface{}) map[string]interface{} {

	result := make(map[string]interface{})

	st := reflect.TypeOf(*&object)

	num := st.NumField()

	// for 1
	for i := 0; i < num; i++ {
		item := st.Field(i)

		// for in data
		for v, _ := range data {
			fmt.Println("item.........", data)
			// check theo tag
			if item.Tag.Get("json") == v {
				// switch
				switch item.Type.Kind() {
				case reflect.Int:
					format := fmt.Sprintf("%d", data[v])
					fmt.Println("format....", format)
					result[item.Name], _ = strconv.Atoi(format)
				case reflect.String:
					result[item.Name] = fmt.Sprintf("%v", data[v])
				default:
				} // end switch

			} // end if check name

		} // end for data

	} // end for 1
	fmt.Println("data v :", result)
	return result
}

// New builds a new Query, given the table and primary key
func New(t string, pk string) *Query {
	// If we have no db, return nil
	if database == nil {
		return nil
	}
	q := &Query{
		tablename:  t,
		primarykey: pk,
	}

	return q
}

// Insert inserts a record in the database
func (q *Query) Insert(params map[string]interface{}) (int64, error) {

	// Insert and retrieve ID in one step from db
	sql := q.formatInsertSQL(params)

	if Debug {
		fmt.Printf("INSERT SQL:%s %v\n", sql, valuesFromParams(params))
	}

	id, err := database.Insert(sql, valuesFromParams(params)...)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// Insert a object in the database
func (q *Query) SaveObject(object interface{}) (int64, error) {
	var params = make(map[string]interface{})
	////--- Extract Value without specifying Type
	val := reflect.Indirect(reflect.ValueOf(object))
	for i := 0; i < val.Type().NumField(); i++ {
		fmt.Println("value field :" , val.Field(i))
		// create map param
		if val.Field(i).IsValid(){
			// switch
			switch val.Field(i).Type().Kind() {
			case reflect.Int:
				params[val.Type().Field(i).Tag.Get("json")] = val.Field(i).Int()
				//result[item.Name] = val
			case reflect.String:
				params[val.Type().Field(i).Tag.Get("json")] = val.Field(i).String()
			default:
			} // end switch

		}
	}
	// Insert and retrieve ID in one step from db
	sql := q.formatInsertSQL(params)

	if Debug {
		fmt.Printf("INSERT SQL:%s %v\n", sql, valuesFromParams(params))
	}

	id, err := database.Insert(sql, valuesFromParams(params)...)
	if err != nil {
		return 0, err
	}

	return id, nil
}


func (q *Query) formatInsertSQL(params map[string]interface{}) string {
	var cols, vals []string

	for i, k := range sortedParamKeys(params) {
		cols = append(cols, database.QuoteField(k))
		vals = append(vals, database.Placeholder(i+1))
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES(%s)", q.tablename, strings.Join(cols, ","), strings.Join(vals, ","))

	return query
}

// Update one model specified in this query - the column names MUST be verified in the model
func (q *Query) Update(params map[string]interface{}) error {

	return q.UpdateAll(params)
}
func (q *Query) UpdateObject(object interface{}) error {
	var params = make(map[string]interface{})
	val := reflect.Indirect(reflect.ValueOf(object))
	for i := 0; i < val.Type().NumField(); i++ {
		// create map param
		if val.Field(i).IsValid(){
			// switch
			switch val.Field(i).Type().Kind() {
			case reflect.Int:
				params[val.Type().Field(i).Tag.Get("json")] = val.Field(i).Int()
			case reflect.String:
				params[val.Type().Field(i).Tag.Get("json")] = val.Field(i).String()
			default:
			} // end switch
		}
	}
	return q.UpdateAll(params)
}

// UpdateAll updates all models specified in this relation
func (q *Query) UpdateAll(params map[string]interface{}) error {
	// Create sql for update from ALL params
	q.UpdateSql(fmt.Sprintf("UPDATE %s SET %s", q.table(), querySQL(params)))

	q.args = append(valuesFromParams(params), q.args...)

	if Debug {
		fmt.Printf("UPDATE SQL:%s\n%v\n", q.QueryString(), valuesFromParams(params))
	}
	test, err := q.Result()
	fmt.Println("test sql...", test)
	return err
}

// DeleteAll delets *all* models specified in this relation
func (q *Query) DeleteAll() error {

	q.Select(fmt.Sprintf("DELETE FROM %s", q.table()))

	if Debug {
		fmt.Printf("DELETE SQL:%s <= %v\n", q.QueryString(), q.args)
	}

	// Execute
	_, err := q.Result()

	return err
}

// Count fetches a count of model objects (executes SQL).
func (q *Query) Count() (int64, error) {
	// Store the previous select and set
	s := q.sel
	countSelect := fmt.Sprintf("SELECT COUNT(%s) FROM %s", q.pk(), q.table())
	q.Select(countSelect)
	o := strings.Replace(q.order, "ORDER BY ", "", 1)
	q.order = ""

	// Fetch count from db for our sql with count select and no order set
	var count int64
	rows, err := q.Rows()
	if err != nil {
		return 0, fmt.Errorf("Error querying database for count: %s\nQuery:%s", err, q.QueryString())
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&count)
		if err != nil {
			return 0, err
		}
	}

	// Reset select after getting count query
	q.Select(s...)
	q.Order(o)
	q.reset()

	return count, err
}

// Result executes the query against the database, returning sql.Result, and error (no rows)
// (Executes SQL)
func (q *Query) Result() (sql.Result, error) {
	fmt.Println("query result....", q.QueryString())
	results, err := database.Exec(q.QueryString(), q.args...)
	return results, err
}

// Rows executes the query against the database, and return the sql rows result for this query
func (q *Query) Rows() (*sql.Rows, error) {
	results, err := database.Query(q.QueryString(), q.args...)
	return results, err
}

// FirstResult executes the SQL and returrns the first result
func (q *Query) FirstResult() (Result, error) {

	// Set a limit on the query
	q.Limit(1)

	// Fetch all results (1)
	results, err := q.Results()
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("No results found for Query:%s", q.QueryString())
	}

	// Return the first result
	return results[0], nil
}

// Results returns an array of results
func (q *Query) Results() ([]Result, error) {

	// Make an empty result set map
	var results []Result
	rows, err := q.Rows()

	if err != nil {
		return results, fmt.Errorf("Error querying database for rows: %s\nQUERY:%s", err, q)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return results, fmt.Errorf("Error fetching columns: %s\nQUERY:%s\nCOLS:%s", err, q, cols)
	}
	for rows.Next() {
		result, err := scanRow(cols, rows)
		if err != nil {
			return results, fmt.Errorf("Error fetching row: %s\nQUERY:%s\nCOLS:%s", err, q, cols)
		}
		results = append(results, result)
	}

	return results, nil
}

// QueryString builds a query string to use for results
func (q *Query) QueryString() string {
	if q.sql == "" {
		selectSlice := make([]string, len(q.sel))
		for i, v := range q.sel {
			selectSlice[i] = fmt.Sprintf("`%s`", trim(v))
		}
		selectSql := ""
		if len(q.sel) <= 0 {
			selectSql = fmt.Sprintf("SELECT %s.* FROM %s", q.table(), q.table())
		} else {
			selectSql = fmt.Sprintf("SELECT %s FROM %s", strings.Join(selectSlice, ","), q.table())
		}
		fmt.Println("q.update len :", len(q.update))
		if len(q.update) > 0{
			selectSql = q.update
		}
		fmt.Println("select sql..", selectSql)

		q.sql = fmt.Sprintf("%s %s %s %s %s %s %s %s", selectSql, q.join, q.where, q.group, q.having, q.order, q.offset, q.limit)
		q.sql = strings.TrimRight(q.sql, " ")
		q.sql = strings.Replace(q.sql, "  ", " ", -1)
		q.sql = strings.Replace(q.sql, "   ", " ", -1)
		fmt.Println(" sql :", q.sql)

		// Replace ? with whatever placeholder db prefers
		q.replaceArgPlaceholders()

		q.sql = q.sql + ";"
	}

	return q.sql
}

// Limit sets the sql LIMIT with an int
func (q *Query) Limit(limit int) *Query {
	q.limit = fmt.Sprintf("LIMIT %d", limit)
	q.reset()
	return q
}

// Offset sets the sql OFFSET with an int
func (q *Query) Offset(offset int) *Query {
	q.offset = fmt.Sprintf("OFFSET %d", offset)
	q.reset()
	return q
}

// Where defines a WHERE clause on SQL - Additional calls add WHERE () AND () clauses
func (q *Query) Where(args ...interface{}) *Query {
	var paramSlice []string
	if args != nil {
		for _, param := range args {
			paramSlice = append(paramSlice, param.(string))
		}
	}
	if len(q.where) > 0 {
		q.where = fmt.Sprintf("%s AND (%s)", q.where, strings.Join(paramSlice, ""))
	} else {
		q.where = fmt.Sprintf(" WHERE (%s)", strings.Join(paramSlice, ""))
	}
	q.reset()
	return q
}

// OrWhere defines a where clause on SQL - Additional calls add WHERE () OR () clauses
func (q *Query) OrWhere(args ...interface{}) *Query {

	var paramSlice []string
	if args != nil {
		for _, param := range args {
			paramSlice = append(paramSlice, param.(string))
		}
	}
	if len(q.where) > 0 {
		q.where = fmt.Sprintf("%s OR (%s)", q.where, strings.Join(paramSlice, ""))
	} else {
		q.where = fmt.Sprintf("WHERE (%s)", strings.Join(paramSlice, ""))
	}

	q.reset()
	return q
}

// WhereIn adds a Where clause which selects records IN() the given array
func (q *Query) WhereIn(col string, IDs []int64) *Query {
	// Return no results, so that when chaining callers
	// don't have to check for empty arrays
	if len(IDs) == 0 {
		q.Limit(0)
		q.reset()
		return q
	}

	in := ""
	for _, ID := range IDs {
		in = fmt.Sprintf("%s%d,", in, ID)
	}
	in = strings.TrimRight(in, ",")
	sql := fmt.Sprintf("%s IN (%s)", col, in)

	if len(q.where) > 0 {
		q.where = fmt.Sprintf("%s AND (%s)", q.where, sql)
	} else {
		q.where = fmt.Sprintf("WHERE (%s)", sql)
	}

	q.reset()
	return q
}

func (q *Query) Join(otherModel string, colJoinModelTable string, colJoinOtherTable string) *Query {
	modelTable := q.tablename

	joinTable := fmt.Sprintf("%s", otherModel)

	sql := fmt.Sprintf("INNER JOIN %s ON %s."+colJoinModelTable+" = %s."+colJoinOtherTable, database.QuoteField(joinTable), database.QuoteField(modelTable), database.QuoteField(joinTable))

	if len(q.join) > 0 {
		q.join = fmt.Sprintf("%s %s", q.join, sql)
	} else {
		q.join = fmt.Sprintf("%s", sql)
	}

	fmt.Println("q join.....", q.join)
	q.reset()
	return q
}

// Order defines ORDER BY sql
func (q *Query) Order(sql string) *Query {
	if sql == "" {
		q.order = ""
	} else {
		q.order = fmt.Sprintf("ORDER BY %s", sql)
	}
	q.reset()

	return q
}

// Group defines GROUP BY sql
func (q *Query) Group(sql string) *Query {
	if sql == "" {
		q.group = ""
	} else {
		q.group = fmt.Sprintf("GROUP BY %s", sql)
	}
	q.reset()
	return q
}

// Having defines HAVING sql
func (q *Query) Having(sql string) *Query {
	if sql == "" {
		q.having = ""
	} else {
		q.having = fmt.Sprintf("HAVING %s", sql)
	}
	q.reset()
	return q
}

// Select defines SELECT  sql
func (q *Query) Select(field ...string) *Query {
	q.sel = field
	q.reset()
	return q
}
// Select defines Update  sql
func (q *Query) UpdateSql(field string) *Query {
	q.update = field
	q.reset()
	return q
}

// Clear sql/query caches
func (q *Query) reset() {
	// clear stored sql
	q.sql = ""
}

// Ask model for primary key name to use
func (q *Query) pk() string {
	return database.QuoteField(q.primarykey)
}

// Ask model for table name to use
func (q *Query) table() string {
	return database.QuoteField(q.tablename)
}

// Replace ?
func (q *Query) replaceArgPlaceholders() {
	// Match ? and replace with argument placeholder from database
	for i := range q.args {
		q.sql = strings.Replace(q.sql, "?", database.Placeholder(i+1), 1)
	}
}

// Sorts the param names given - map iteration order is explicitly random in Go
// but we need params in a defined order to avoid unexpected results.
func sortedParamKeys(params map[string]interface{}) []string {
	sortedKeys := make([]string, len(params))
	i := 0
	for k := range params {
		sortedKeys[i] = k
		i++
	}
	sort.Strings(sortedKeys)

	return sortedKeys
}

// Generate a set of values for the params in order
func valuesFromParams(params map[string]interface{}) []interface{} {
	var values []interface{}
	for _, key := range sortedParamKeys(params) {
		values = append(values, params[key])
	}
	return values
}

// Used for update statements, turn params into sql i.e. "col"=?
func querySQL(params map[string]interface{}) string {
	var output []string
	for _, k := range sortedParamKeys(params) {
		output = append(output, fmt.Sprintf("%s=?", database.QuoteField(k)))
	}
	return strings.Join(output, ",")
}

func scanRow(cols []string, rows *sql.Rows) (Result, error) {
	// We return a map[string]interface{} for each row scanned
	result := Result{}
	values := make([]interface{}, len(cols))
	for i := 0; i < len(cols); i++ {
		var col interface{}
		values[i] = &col
	}
	// Scan results into these interfaces
	err := rows.Scan(values...)
	if err != nil {
		return nil, fmt.Errorf("Error scanning row: %s", err)
	}

	for i := 0; i < len(cols); i++ {
		v := *values[i].(*interface{})
		if values[i] != nil {
			switch v.(type) {
			default:
				result[cols[i]] = v
			case bool:
				result[cols[i]] = v.(bool)
			case int:
				result[cols[i]] = int64(v.(int))
			case []byte: // text cols are given as bytes
				result[cols[i]] = string(v.([]byte))
			case int64:
				result[cols[i]] = v.(int64)
			}
		}

	}
	return result, nil
}
func trim(str string) string {
	re := regexp.MustCompile(`[\s]+`)
	// replace multi space = 1 space
	str = re.ReplaceAllString(str, " ")

	return strings.TrimSpace(str)
}
