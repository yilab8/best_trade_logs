package main

import (
	"flag"
	"os"
)

type config struct {
	Port            string
	MongoURI        string
	MongoDatabase   string
	MongoCollection string
}

func loadConfig() (config, error) {
	cfg := config{
		Port:            getEnv("PORT", "8080"),
		MongoURI:        os.Getenv("MONGO_URI"),
		MongoDatabase:   os.Getenv("MONGO_DB"),
		MongoCollection: os.Getenv("MONGO_COLLECTION"),
	}

	flag.StringVar(&cfg.Port, "port", cfg.Port, "HTTP port to listen on")
	flag.StringVar(&cfg.MongoURI, "mongo-uri", cfg.MongoURI, "MongoDB connection URI")
	flag.StringVar(&cfg.MongoDatabase, "mongo-db", cfg.MongoDatabase, "MongoDB database name")
	flag.StringVar(&cfg.MongoCollection, "mongo-collection", cfg.MongoCollection, "MongoDB collection name")
	flag.Parse()

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.MongoCollection == "" {
		cfg.MongoCollection = "trades"
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
