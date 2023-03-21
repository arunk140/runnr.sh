package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/joho/godotenv"
)

var sessionHistory []RMessage
var maxCounter int
var currentWorkingDir string

func appendToSessionHistory(role RFrom, content string) {
	sessionHistory = append(sessionHistory, RMessage{
		Role:    role,
		Content: content,
	})
	file, err := os.Create("session_history.json")
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()
	_, err = file.WriteString(historyToString(sessionHistory))
	if err != nil {
		log.Fatalln(err)
	}
}

func historyToString(h []RMessage) string {
	bytes, err := json.Marshal(h)
	if err != nil {
		log.Fatalln(err)
	}
	return string(bytes)
}

func executeCommandWithBash(command string) (string, int, string) {
	cmd := exec.Command("bash", "-c", command+" && pwd")
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	cmd.Dir = currentWorkingDir
	err := cmd.Run()
	if err != nil {
		log.Printf("error: %v", err)
	}
	exitCode := cmd.ProcessState.ExitCode()
	if exitCode == 0 {
		lines := strings.Split(outb.String(), "\n")
		currentWorkingDir = lines[len(lines)-2]

		return strings.Join(lines[:len(lines)-2], "\n"), 0, errb.String()
	}

	return outb.String(), exitCode, errb.String()
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
	log.Printf("Making API Call: %d", counter)
	apiRes := makeOpenAIAPICall()
	appendToSessionHistory(API, apiRes)
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
	response, err := jsonparser.GetString(bodyText, "choices", "[0]", "message", "content")
	if err != nil {
		log.Fatal(err)
	}
	return response
}

func runCommand(command string, counter int) string {
	log.Println("Current Working Directory: ", currentWorkingDir)
	log.Printf("Running command: %s", command)

	cmdRes := fmt.Sprintf("Running command: %s", command)
	command = strings.TrimSpace(command)
	out, exitCode, err := executeCommandWithBash(command)
	if err != "" {
		cmdRes += fmt.Sprintf("\nError: %s", err)
	}

	cmdRes += fmt.Sprintf("\nOutput: %s", out)
	cmdRes += fmt.Sprintf("\nExit Code: %d", exitCode)
	cmdRes += "\nReply with \"DONE\" if the above output completes the give task. Else reply with \"CONTINUE|{COMMAND}\" with the next step."

	appendToSessionHistory(Machine, cmdRes)

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

	prefix := `Pretend to be a Linux Expert managing a Linux Server. A user will give you a task. Your main purpose is to return Linux terminal Commands and also validate thier outputs.
Reply with "DONE" only when the task provided by the User is complete. The user will provide you with the outputs of the commands you provide. Only reply with the command and no explainations.
Return Linux Terminal Commands (one at a time) in the format -
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

	appendToSessionHistory(System, prefix)
	sysinfo, sysinfoExitCode, sysinfoErr := executeCommandWithBash("./sysinfo.sh")
	if sysinfoExitCode != 0 {
		log.Printf("sysinfo.sh error: %v", sysinfoErr)
	} else {
		appendToSessionHistory(Machine, sysinfo)
	}

	appendToSessionHistory(User, "TASK: "+initialInput)
	log.Println("Starting...")

	_ = apiCall(counter)

	fmt.Println("################################")
	fmt.Println(historyToString(sessionHistory))
}
