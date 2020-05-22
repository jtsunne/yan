package main

type Host struct {
	Hostname string
	Port     string
	Username string
	Tasks    map[string]Task
}

type Task struct {
	Type  string
	Value string
}

type Config struct {
	Hosts map[string]Host
}
