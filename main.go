package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	dbUser     = os.Getenv("DB_USER")
	dbPassword = os.Getenv("DB_PASSWORD")
	dbHost     = os.Getenv("DB_HOST")
	dbName     = os.Getenv("DB_NAME")
	tableSizeGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mysql_table_storage_bytes",
			Help: "The size of the MySQL table in bytes (Data_length + Index_length).",
		},
		[]string{"table_name", "database_name"},
	)
	customRegistry = prometheus.NewRegistry()
)

func connectDB() (*sql.DB, error) {
	dsn := dbUser + ":" + dbPassword + "@tcp(" + dbHost + ")/" + dbName
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func updateMetrics() {
	db, err := connectDB()
	if err != nil {
		log.Printf("ERROR: Could not connect to DB: %v", err)
		return
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("ERROR closing db: %v", err)
		}
	}()

	query := `
		SELECT 
			TABLE_SCHEMA AS 'database',
			TABLE_NAME AS 'table',
			(DATA_LENGTH + INDEX_LENGTH) AS 'size'
		FROM
		  information_schema.TABLES
		HAVING size > 0
	`
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("ERROR querying table sizes: %v", err)
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("ERROR closing rows: %v", err)
		}
	}()

	var count int
	for rows.Next() {
		var databaseName string
		var tableName string
		var tableSize float64

		if err := rows.Scan(&databaseName, &tableName, &tableSize); err != nil {
			log.Printf("ERROR scanning row: %v", err)
			continue
		}

		tableSizeGauge.With(prometheus.Labels{"table_name": tableName, "database_name": databaseName}).Set(tableSize)
		count++
	}

	if err := rows.Err(); err != nil {
		log.Printf("ERROR iterating rows: %v", err)
	}

	log.Printf("Successfully updated metrics for %d tables in database '%s'.", count, dbName)
}

func init() {
	customRegistry.MustRegister(tableSizeGauge)
}

func main() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		updateMetrics()
		for range ticker.C {
			updateMetrics()
		}
	}()

	http.Handle("/metrics", promhttp.HandlerFor(
		customRegistry,
		promhttp.HandlerOpts{},
	))

	port := ":9100"
	log.Printf("Starting MySQL Table Size Exporter on http://localhost%s/metrics", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
