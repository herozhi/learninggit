package infsecc

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	TokenPathEnv       string = "SEC_TOKEN_PATH"
	TokenStringEnv     string = "SEC_TOKEN_STRING"
	INFSEC_SEC_USER = "INFSEC_SEC_USER"
	INFSEC_SEC_PSM = "INFSEC_SEC_PSM"
	GET_SEC_TOKEN_STRING_FROM_DAEMON = "GET_SEC_TOKEN_STRING_FROM_DAEMON"
	UnixDomainSocketPath = "/opt/tmp/sock/.unix_sock_agent_1234567890.sock"
	UpdateIntervalSecs int64  = 180
)

type CommandTag struct {
	Cmd uint8 `json:"cmd"`
	PSM string `json:"psm"`
	User string `json:"user"`
}

var (
	tokenPath   string
	tokenStr    string
	token       string
	infsecUser string
	infsecPSM string
	getTokenFromDaemon string
	daemonToken string
	pathToken string
	updateTime  int64
	lock      sync.Mutex
	mutex     sync.Mutex
	routineMutex     sync.Mutex
	variableAssignLock sync.Mutex
	variableAssignMutex sync.Mutex
)

func init() {
	tokenPath = os.Getenv(TokenPathEnv)
	tokenStr = os.Getenv(TokenStringEnv)
	getTokenFromDaemon = os.Getenv(GET_SEC_TOKEN_STRING_FROM_DAEMON)
	infsecPSM = os.Getenv(INFSEC_SEC_PSM)
	infsecUser = os.Getenv(INFSEC_SEC_USER)
	if getTokenFromDaemon == "1" && (infsecPSM != "" || infsecUser != "") {
		go daemonRoutine()
	} else if "" != tokenPath {
		updateToken()
		go tokenPathRoutine()
	}
}

func SetTokenPath(s string) {
	mutex.Lock()
	tokenPath = s
	mutex.Unlock()
}

func SetTokenStr(s string) {
	lock.Lock()
	tokenStr = s
	lock.Unlock()
}

func updateToken() error {
	mutex.Lock()
	defer mutex.Unlock()
	if tokenPath == "" {
		return fmt.Errorf("gdpr tokenPath is empty!")
	}
	_, err := os.Stat(tokenPath)
	if err != nil {
		return err
	}
	tokenbytes, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return err
	}
	variableAssignMutex.Lock()
	pathToken = string(tokenbytes)
	variableAssignMutex.Unlock()
	return nil
}

func GetToken(forceupdate bool) (string, error) {
	lock.Lock()
	if tokenStr != "" {
		tokenString := tokenStr
		lock.Unlock()
		return tokenString, nil
	}
	lock.Unlock()
	now := time.Now().Unix()
	if forceupdate || now-updateTime > UpdateIntervalSecs {
		routineMutex.Lock()
		if forceupdate || now-updateTime > UpdateIntervalSecs {
			updateTime = now
			if getTokenFromDaemon == "1" && (infsecPSM != "" || infsecUser != "") {
				if daemonToken == "" {
					pullTokenFromDaemon()
				}
				variableAssignLock.Lock()
				token = daemonToken
				variableAssignLock.Unlock()
			} else {
				// use the other goroutine to update token without invoking updateToken() directly
				variableAssignMutex.Lock()
				token = pathToken
				variableAssignMutex.Unlock()
			}
		}
		routineMutex.Unlock()
	}
	if token == "" {
		return "", fmt.Errorf("token is empty!")
	}
	return token, nil
}

// Decode JWT specific base64url encoding with padding stripped
func decodeSegment(seg string) ([]byte, error) {
	if l := len(seg) % 4; l > 0 {
		seg += strings.Repeat("=", 4-l)
	}

	return base64.URLEncoding.DecodeString(seg)
}

func ParseToken(tokenString string) (*Identity, error) {
	identity := &Identity{}
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format!")
	}
	claimBytes, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("fail to decode token!")
	}
	dec := json.NewDecoder(bytes.NewBuffer(claimBytes))
	err = dec.Decode(identity)
	if err != nil {
		return nil, err
	}
	return identity, nil
}

func daemonRoutine()  {
	for {
		pullTokenFromDaemon()
		time.Sleep(180 * time.Second)
	}
}

func pullTokenFromDaemon() {
	var unixAddr *net.UnixAddr
	unixAddr, _ = net.ResolveUnixAddr("unix", UnixDomainSocketPath)

	for i := 0; i < 1; i++ {
		conn, err := net.DialUnix("unix", nil, unixAddr)
		if nil != err {
			continue
		}

		var cmd CommandTag
		cmd.PSM = infsecPSM
		cmd.Cmd = 1
		cmd.User = infsecUser

		b, err := json.Marshal(cmd)
		if nil == err {
			conn.Write(b)
			conn.Write([]byte("\n"))
			reader := bufio.NewReader(conn)
			if nil != reader {
				msg, err := reader.ReadString('\n')
				if nil == err {
					if strings.Contains(msg, "\n") {
						variableAssignLock.Lock()
						daemonToken = strings.Split(msg, "\n")[0]
						variableAssignLock.Unlock()
					}
				}
			}
		}
		conn.Close()
	}
}

func tokenPathRoutine()  {
	for {
		updateToken()
		time.Sleep(180 * time.Second)
	}
}
