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
	"yan/config"
)

func executeCmd(command, keyTask, hostKey string, host config.Host, config *ssh.ClientConfig) string {
	port := "22"
	if host.Port != "" {
		port = host.Port
	}
	conn, _ := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.Hostname, port), config)
	session, _ := conn.NewSession()
	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Run(command)

	return fmt.Sprintf("%s -> %s -> %s", hostKey, keyTask, strings.TrimSpace(stdoutBuf.String()))
}

func main() {
	var yamConfig config.Config
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
		go func(host2 config.Host) {
			userName := "root"
			if host2.Username != "" {
				userName = host2.Username
			}
			c := &ssh.ClientConfig{
				User:            userName,
				Auth:            []ssh.AuthMethod{ssh.PublicKeysCallback(agentClient.Signers)},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}
			for keyTask, task := range host.Tasks {
				log.Println(keyHost, keyTask)
				results <- executeCmd(task.Value, keyTask, keyHost, host2, c)
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
