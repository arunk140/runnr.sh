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
	cmd := exec.Command("bash", "-c", "./run.sh \""+command+"\"")
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cmd.Stdin = os.Stdin

	cmd.Dir = currentWorkingDir
	err := cmd.Run()
	if err != nil {
		log.Printf("error: %v", err)
	}
	exitCode := cmd.ProcessState.ExitCode()
	if exitCode == 0 {
		lines := strings.Split(outb.String(), "\n")
		currentWorkingDir = lines[len(lines)-2]

		return strings.Join(lines[:len(lines)-2], "\n"), exitCode, errb.String()
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
	out, _, err := executeCommandWithBash(command)
	if err != "" {
		cmdRes += fmt.Sprintf("\nError: %s", err)
	}
	if out != "" {
		cmdRes += fmt.Sprintf("\nOutput: %s", out)
	}
	// cmdRes += fmt.Sprintf("\nExit Code: %d", exitCode) // Exit code is already included in the output
	cmdRes += "\nReply with \"DONE\" if the above output completes the give task. Else reply with \"CONTINUE|{COMMAND}\" with the next step."

	appendToSessionHistory(Machine, cmdRes)

	apiCall(counter + 1)
	return cmdRes
}

func main() {
	log.SetFlags(0)
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	var (
		counter      int
		initialInput string
	)

	prefix := `In this exercise, you will act as a Linux expert who manages a Linux server. You will be given tasks by a user, and your job is to provide Linux terminal commands to complete the tasks. Your task is to provide the terminal commands only without any explanations.
You must use the format "CONTINUE|{COMMAND}" to provide the terminal command to the user. If you want to validate the command's output, you should include the keyword "CONTINUE" in all caps before the command. You will receive the output of the command you provided from the user.
Your goal is to complete the task provided by the user, and you should reply with "DONE" only when the task is complete. Remember to only reply with "DONE" or "CONTINUE|{COMMAND}" and no other information.`

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

	initialInput = strings.TrimSpace(strings.TrimSuffix(userData, "\n"))
	if initialInput == "" {
		log.Fatalln("Input Command is empty")
	}

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

	fmt.Println("Task Completed!")
}
