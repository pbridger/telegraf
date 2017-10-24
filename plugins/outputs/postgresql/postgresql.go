package postgresql

import (
	"database/sql"
	"fmt"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	"strings"
)

type Postgresql struct {
	db                *sql.DB
	Address           string
	CreateTables      bool
	TagsAsForeignkeys bool
	Tables            map[string]bool
	SchemaTag         string
}

func (p *Postgresql) Connect() error {
	db, err := sql.Open("pgx", p.Address)
	if err != nil {
		return err
	}
	p.db = db
	p.Tables = make(map[string]bool)

	return nil
}

func (p *Postgresql) Close() error {
	return p.db.Close()
}

func (p *Postgresql) SampleConfig() string { return "" }
func (p *Postgresql) Description() string  { return "Send metrics to PostgreSQL" }

func (p *Postgresql) generateCreateTable(metric telegraf.Metric) string {
	var columns []string
	var pk []string

	pk = append(pk, "time")
	columns = append(columns, "time timestamptz")

	for column, _ := range metric.Tags() {
		pk = append(pk, column)
		columns = append(columns, fmt.Sprintf("%s text", column))
	}

	var datatype string
	for column, v := range metric.Fields() {
		switch v.(type) {
		case int64:
			datatype = "int8"
		case float64:
			datatype = "float8"
		}
		columns = append(columns, fmt.Sprintf("%s %s", column, datatype))
	}

	sql := fmt.Sprintf("CREATE TABLE %s(%s,PRIMARY KEY(%s))", metric.Name(), strings.Join(columns, ","), strings.Join(pk, ","))
	fmt.Println(sql)
	return sql
}

func (p *Postgresql) generateInsert(metric telegraf.Metric) (string, []interface{}) {
	var columns []string
	var values []interface{}

	columns = append(columns, "time")
	values = append(values, metric.Time())

	for column, value := range metric.Tags() {
		columns = append(columns, column)
		values = append(values, value)
	}

	for column, value := range metric.Fields() {
		columns = append(columns, column)
		values = append(values, value)
	}

	var placeholder []string
	for i := 1; i <= len(values); i++ {
		placeholder = append(placeholder, fmt.Sprintf("$%d", i))
	}

	sql := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", metric.Name(), strings.Join(columns, ","), strings.Join(placeholder, ","))
	return sql, values
}

func (p *Postgresql) writeMetric(metric telegraf.Metric) error {
	tableName := metric.Name()

	if p.Tables[tableName] == false {
		createStmt := p.generateCreateTable(metric)
		_, err := p.db.Exec(createStmt)
		if err != nil {
			fmt.Println("Error creating table", err)
			return err
		}
		p.Tables[tableName] = true
	}

	sql, values := p.generateInsert(metric)
	_, err := p.db.Exec(sql, values...)
	if err != nil {
		fmt.Println("Error during insert", err)
		return err
	}

	return nil
}

func (p *Postgresql) Write(metrics []telegraf.Metric) error {
	for _, metric := range metrics {
		err := p.writeMetric(metric)
		if err != nil {
			return err
		}
	}
	return nil
}

func init() {
	outputs.Add("postgresql", func() telegraf.Output { return &Postgresql{} })
}
