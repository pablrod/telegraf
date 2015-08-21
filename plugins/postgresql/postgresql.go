package postgresql

import (
    "fmt"
	"database/sql"

	"github.com/pablrod/telegraf/plugins"

	_ "github.com/lib/pq"
)

type Query struct {
    Identifier string
    Sql string
    Tags []string
    Values []string
}

type Server struct {
	Address   string
	Databases []string
    Queries []string
}

type Postgresql struct {
	Servers []*Server
    Queriesdefinition []Query
}

var sampleConfig = `
# specify servers via an array of tables
[[postgresql.servers]]

# specify address via a url matching:
#   postgres://[pqgotest[:password]]@localhost?sslmode=[disable|verify-ca|verify-full]
# or a simple string:
#   host=localhost user=pqotest password=... sslmode=...
# 
# All connection parameters are optional. By default, the host is localhost
# and the user is the currently running user. For localhost, we default
# to sslmode=disable as well.
# 

address = "sslmode=disable"

# A list of databases to pull metrics about. If not specified, metrics for all
# databases are gathered.

# databases = ["app_production", "blah_testing"]

# [[postgresql.servers]]
# address = "influx@remoteserver"

# A list of queries to execute for this server
# queries = ["indexes_size", "tables_size"]

# Queries definition
# [[postgresql.queriesdefinition]]
# identifier = "tables_size"
# sql = "select (select nspname from pg_catalog.pg_namespace where oid = pg_class.relnamespace) as schema, relname as table, pg_total_relation_size(oid) as bytes from pg_catalog.pg_class where relnamespace not in (select oid from pg_catalog.pg_namespace where nspname like 'pg_%') and pg_total_relation_size(oid) > 1024*1024*1024 order by pg_total_relation_size(oid) desc"
# tags =["schema", "table"]
# values = ["bytes"]
# [[postgresql.queriesdefinition]]
# identifier = "indexes_size"
# sql = "select * from pg_class"
`

func (p *Postgresql) SampleConfig() string {
	return sampleConfig
}

func (p *Postgresql) Description() string {
	return "Read metrics from one or many postgresql servers"
}

var localhost = &Server{Address: "sslmode=disable"}

func (p *Postgresql) Gather(acc plugins.Accumulator) error {

	if len(p.Servers) == 0 {
		p.gatherServer(localhost, acc)
		return nil
	}

	//for _, serv := range p.Servers {
    //		err := p.gatherServer(serv, acc)
    //		if err != nil {
    //			return err
    //		}
	//}

	for _, serv := range p.Servers {
		err := p.gatherServerQueries(serv, acc, &p.Queriesdefinition)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Postgresql) gatherServer(serv *Server, acc plugins.Accumulator) error {
	if serv.Address == "" || serv.Address == "localhost" {
		serv = localhost
	}

	db, err := sql.Open("postgres", serv.Address)
	if err != nil {
		return err
	}

	defer db.Close()

	if len(serv.Databases) == 0 {
		rows, err := db.Query(`SELECT * FROM pg_stat_database`)
		if err != nil {
			return err
		}

		defer rows.Close()

		for rows.Next() {
			err := p.accRow(rows, acc, serv.Address)
			if err != nil {
				return err
			}
		}

		return rows.Err()
	} else {
		for _, name := range serv.Databases {
			row := db.QueryRow(`SELECT * FROM pg_stat_database WHERE datname=$1`, name)

			err := p.accRow(row, acc, serv.Address)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Postgresql) gatherServerQueries(serv *Server, acc plugins.Accumulator, queries *[]Query) error {
	if serv.Address == "" || serv.Address == "localhost" {
		serv = localhost
	}

    fmt.Println(serv.Queries)

    queries_map := make(map[string]Query)
    for _, query := range *queries {
        queries_map[query.Identifier] = query 
    }

    for _, database_name := range serv.Databases {
        fmt.Println(serv.Address)
        fmt.Println(database_name)
        db, err := sql.Open("postgres", serv.Address)
        if err != nil {
            fmt.Println(err)
            return err
        }
        defer db.Close()
        for _, query_id := range serv.Queries {
            fmt.Println("Consulta: ")
            fmt.Println(queries_map[query_id].Sql)
            rows, err := db.Query(queries_map[query_id].Sql)
            if err != nil {
                return err
            }

            defer rows.Close()
            
            for rows.Next() {
                // Acumulate row
                var schema_name string
                var table_name string
                var size_in_bytes float64
                rows.Scan(&schema_name, &table_name, &size_in_bytes)    
                tags := map[string]string{"db": database_name, "schema": schema_name, "table": table_name}
                acc.Add("size", size_in_bytes, tags) 
            }
        }
    }
    
	return nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (p *Postgresql) accRow(row scanner, acc plugins.Accumulator, server string) error {
	var ignore interface{}
	var name string
	var commit, rollback, read, hit int64
	var returned, fetched, inserted, updated, deleted int64
	var conflicts, temp_files, temp_bytes, deadlocks int64
	var read_time, write_time float64

	err := row.Scan(&ignore, &name, &ignore,
		&commit, &rollback,
		&read, &hit,
		&returned, &fetched, &inserted, &updated, &deleted,
		&conflicts, &temp_files, &temp_bytes,
		&deadlocks, &read_time, &write_time,
		&ignore,
	)

	if err != nil {
		return err
	}

	tags := map[string]string{"server": server, "db": name}

	acc.Add("xact_commit", commit, tags)
	acc.Add("xact_rollback", rollback, tags)
	acc.Add("blks_read", read, tags)
	acc.Add("blks_hit", hit, tags)
	acc.Add("tup_returned", returned, tags)
	acc.Add("tup_fetched", fetched, tags)
	acc.Add("tup_inserted", inserted, tags)
	acc.Add("tup_updated", updated, tags)
	acc.Add("tup_deleted", deleted, tags)
	acc.Add("conflicts", conflicts, tags)
	acc.Add("temp_files", temp_files, tags)
	acc.Add("temp_bytes", temp_bytes, tags)
	acc.Add("deadlocks", deadlocks, tags)
	acc.Add("blk_read_time", read_time, tags)
	acc.Add("blk_write_time", read_time, tags)

	return nil
}

func init() {
	plugins.Add("postgresql", func() plugins.Plugin {
		return &Postgresql{}
	})
}
