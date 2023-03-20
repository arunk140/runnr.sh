package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/joho/godotenv"
)

var sessionHistory []RMessage
var maxCounter int

func historyToString(h []RMessage) string {
	bytes, err := json.Marshal(h)
	if err != nil {
		log.Fatalln(err)
	}
	return string(bytes)
}

func parseCounter(strArg string) int {
	var err error
	var num int
	if num, err = strconv.Atoi(strArg); err != nil {
		panic(err)
	}
	return num
}

func apiCall(counter int) string {
	apiRes := makeOpenAIAPICall()
	sessionHistory = append(sessionHistory, RMessage{
		Role:    API,
		Content: apiRes,
	})

	parts := strings.Split(apiRes, "|")
	if parts[0] == "DONE" || counter >= maxCounter {
		return "DONE"
	}
	if len(parts) >= 1 && parts[0] == "CONTINUE" {
		runCommand(parts[1], counter)
	}
	return apiRes
}

func makeOpenAIAPICall() string {
	apiCallBody := OpenAIBody{
		Model:       "gpt-3.5-turbo",
		Messages:    sessionHistory,
		Temperature: 0.7,
	}
	bytes, jErr := json.Marshal(apiCallBody)
	if jErr != nil {
		log.Fatalln(jErr)
	}

	data := strings.NewReader(string(bytes))
	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", data)

	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	response, _ := jsonparser.GetString(bodyText, "choices", "[0]", "message", "content")
	return response
}

func runCommand(command string, counter int) string {
	cmdRes := fmt.Sprintf("Running command: %s", command)
	cmdRes += "Output:\n"
	cmdRes += "hello world\n"
	cmdRes += "\nReply with \"DONE\" if the above output completes the give task. Else reply with \"CONTINUE|{COMMAND}\" with the next step."
	sessionHistory = append(sessionHistory, RMessage{
		Role:    Machine,
		Content: cmdRes,
	})
	apiCall(counter + 1)
	return cmdRes
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	var (
		counter      int
		initialInput string
	)

	prefix := `Pretend to be a Linux Expert managing a Linux Server. A user will give you a task. Your main purpose is to return Linux terminal Commands and also validate thier ouputs.
Reply with "DONE" only when the task provided by the User is complete. The user will provide you with the outputs of the commands you provide. Only reply with the command and no explainations.
Return Linux Terminal Commands in the format -
CONTINUE|{COMMAND}
with the keyword "CONTINUE" in all caps if you want to validate the commands output and {COMMAND} is the terminal command.
Remember - Do not reply with anything other than "CONTINUE|{COMMAND}" or "DONE". If the task is complete reply with "DONE" and nothing else.`

	counter = 1
	if len(os.Args) > 1 {
		maxCounter = parseCounter(os.Args[1])
	} else {
		maxCounter = 10
	}

	log.Print("Input Command: ")

	userData, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		_ = fmt.Errorf(" %v error", err)
		return
	}

	initialInput = strings.TrimSuffix(userData, "\n")

	sessionHistory = append(sessionHistory, RMessage{
		Role:    System,
		Content: prefix,
	})
	sessionHistory = append(sessionHistory, RMessage{
		Role:    User,
		Content: "TASK: " + initialInput,
	})
	_ = apiCall(counter)

	fmt.Println("################################")
	fmt.Println(historyToString(sessionHistory))
}
