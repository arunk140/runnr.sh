package main

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strings"
)

func readExample(filePath string) []RMessage {
	file, err := os.Open(filePath)

	textData := make([]string, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		textData = append(textData, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	fullText := strings.Join(textData, "\n")
	var exampleMessages []RMessage

	json.Unmarshal([]byte(fullText), &exampleMessages)

	for index, message := range exampleMessages {
		// if Role is "system" then delete the message
		if message.Role == "system" {
			exampleMessages = append(exampleMessages[:index], exampleMessages[index+1:]...)
		}
	}

	return exampleMessages
}
