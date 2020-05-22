package main

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

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

func executeCmd(command, keyTask, hostKey string, host Host, config *ssh.ClientConfig) string {
	conn, _ := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.Hostname, host.Port), config)
	session, _ := conn.NewSession()
	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Run(command)

	return fmt.Sprintf("%s -> %s -> %s", hostKey, keyTask, strings.TrimSpace(stdoutBuf.String()))
}

func main() {
	var yamConfig Config
	configFileName := os.Getenv("YAN_CONFIG")
	if configFileName == "" {
		configFileName = "config/yan.yaml"
	}
	configFile, err := ioutil.ReadFile(configFileName)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(configFile, &yamConfig)
	if err != nil {
		panic(err)
	}

	results := make(chan string, 10)
	timeout := time.After(10 * time.Second)
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		log.Fatal("SSH_AUTH_SOCK is not set in ENV")
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		log.Fatalf("Failed to open SSH_AUTH_SOCK: %v", err)
	}

	agentClient := agent.NewClient(conn)

	for keyHost, host := range yamConfig.Hosts {
		host := host
		keyHost := keyHost
		go func(host2 Host) {
			config := &ssh.ClientConfig{
				User:            host2.Username,
				Auth:            []ssh.AuthMethod{ssh.PublicKeysCallback(agentClient.Signers)},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}
			for keyTask, task := range host.Tasks {
				log.Println(keyHost, keyTask)
				results <- executeCmd(task.Value, keyTask, keyHost, host2, config)
			}
		}(host)
	}

	for true {
		select {
		case res := <-results:
			log.Println(res)
		case <-timeout:
			log.Println("Timed out!")
			return
		}
	}
}
