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
	"github.com/fatih/color"
	"github.com/joho/godotenv"
)

var sessionHistory []RMessage
var maxCounter int
var currentWorkingDir string

var totalTokenCount int

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
	cmd.Stdin = os.Stdin

	cmd.Dir = currentWorkingDir
	err := cmd.Run()
	if err != nil {
		color.Red("error: %v", err)
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
	// color.Magenta("Making API Call: %d", counter)
	color.Magenta("Processing ... ")
	apiRes := makeOpenAIAPICall()
	appendToSessionHistory(API, apiRes)
	parts := strings.Split(apiRes, "|")
	if parts[0] == "DONE" || counter >= maxCounter {
		return "DONE"
	}
	if len(parts) >= 1 && parts[0] == "CONTINUE" {
		runCommand(apiRes, counter)
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
		errMsg, _ := jsonparser.GetString(bodyText, "error", "message")
		log.Println(errMsg)
		log.Fatal(err)
	}
	totalTokens, err := jsonparser.GetInt(bodyText, "usage", "total_tokens")
	if err != nil {
		log.Fatal(err)
	}
	totalTokenCount += int(totalTokens)
	return response
}

func runCommand(command string, counter int) string {
	var cmdRes string
	log.Println("Current Working Directory: ", currentWorkingDir)
	cm := strings.Split(command, "CONTINUE|")
	for _, c := range cm {
		c = strings.TrimSpace(c)
		cUpper := strings.ToUpper(c)
		if c != "" && cUpper != "DONE" {
			color.Yellow("Running command: %s", c)
			cmdRes += fmt.Sprintf("Running command: %s", c)
			out, ec, err := executeCommandWithBash(c)
			if err != "" {
				cmdRes += fmt.Sprintf("\nError: %s", err)
			}
			if out != "" {
				cmdRes += fmt.Sprintf("\nOutput: %s", out)
			}
			cmdRes += fmt.Sprintf("\nExit Code: %d", ec)
		}
	}
	// cmdRes += fmt.Sprintf("\nExit Code: %d", exitCode) // Exit code is already included in the output
	cmdRes += "\nReply with \"DONE\" if the above output completes the give task. Else reply with \"CONTINUE|{COMMAND}\" with the next step."
	cmdRes += "\nDo not use nano, vi, vim, emacs, or any other text editor. Or any other command that requires user input."

	appendToSessionHistory(Machine, cmdRes)

	apiCall(counter + 1)
	return cmdRes
}

func main() {
	log.SetFlags(0)
	// log.SetFlags(log.LstdFlags | log.Lshortfile)
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

	color.Green("Input Command: ")

	userData, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		_ = fmt.Errorf(" %v error", err)
		log.Fatal(err)
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

	exampleConv := readExample("examples/files.json")

	for _, e := range exampleConv {
		appendToSessionHistory(e.Role, e.Content)
	}
	initialInput += "\nReply with \"DONE\" if the above output completes the give task. Else reply with \"CONTINUE|{COMMAND}\" with the next step."
	initialInput += "\nDo not use nano, vi, vim, emacs, or any other text editor. Or any other command that requires user input."
	// initialInput += "\nif you create a file, validate the file's content using the command \"CONTINUE|cat {FILE_NAME}\""
	initialInput += "\nStrictly, only return commands - Use only single line commands. Return one command at a time."

	appendToSessionHistory(User, "TASK: "+initialInput)
	log.Println("Starting...")
	totalTokenCount = 0

	_ = apiCall(counter)

	fmt.Println("Task Completed!")
	color.Magenta("Total Tokens Used: %d", totalTokenCount)
	color.Magenta("Cost - $%f", float64(totalTokenCount)/1000*0.002)
}
