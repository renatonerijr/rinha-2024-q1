package db

import (
	"log"
	"os"
	"path/filepath"
	"rinha/domain/model"
	"runtime"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres" // Substitua pelo dialetor do banco de dados que você está usando
	"gorm.io/gorm"
)

func init() {
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)

	err := godotenv.Load(basepath + "/../../.env")
	if err != nil {
		log.Fatalf("Error loading .env files: %v", err)
	}
}

func ConnectDB(env string) *gorm.DB {
	var dsn string
	var db *gorm.DB

	if env != "test" {
		dsn = os.Getenv("DSN")                                // Substitua "DNS" pela chave real do seu arquivo .env
		db, _ = gorm.Open(postgres.Open(dsn), &gorm.Config{}) // Ajuste para o dialetor correto
	} else {
		dsn = os.Getenv("DSN_TEST")                           // Substitua "DNS_TEST" pela chave real do seu arquivo .env
		db, _ = gorm.Open(postgres.Open(dsn), &gorm.Config{}) // Ajuste para o dialetor correto
	}

	if os.Getenv("AUTO_MIGRATE_DB") == "true" {
		db.AutoMigrate(&model.Transaction{})
	}

	return db
}
