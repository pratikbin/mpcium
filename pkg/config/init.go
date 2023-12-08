package config

import (
	"log"

	"github.com/spf13/viper"
)

func InitViperConfig() {
	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")      // optionally look for config in the working directory
	err := viper.ReadInConfig()   // Find and read the config file

	if err != nil { // Handle errors reading the config file
		log.Fatal("Read config failed", err)
		return
	}

	log.Println("Reading config file:", viper.ConfigFileUsed())
	log.Println("Initialized config successfully!")
}
