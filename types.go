package main

type RFrom string

const (
	System  RFrom = "system"
	User    RFrom = "user"
	API     RFrom = "assistant"
	Machine RFrom = "user"
)

type RMessage struct {
	Role    RFrom  `json:"role"`
	Content string `json:"content"`
}

type OpenAIBody struct {
	Model       string     `json:"model"`
	Messages    []RMessage `json:"messages"`
	Temperature float32    `json:"temperature"`
}
