package main

import (
	// "crypto/tls"
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

var hostname string
var port string

func main() {
	readArgs()
	connectionString := "ftp.3700.network:21"
	connection, err := makeConn(connectionString)
	checkError(err)
	defer connection.Close()
	// writeToServer(connection, "AUTH TLS\r\n")
	response, readError := readFromServer(connection)
	checkError(readError)
	fmt.Println(response)
	// connection = tls.Client(connection, &tls.Config{ServerName: "ftp.3700.network"})
	// writeToServer(connection, "USER susmand\r\n")
	// response, readError = readFromServer(connection)
	// checkError(readError)
	// fmt.Println(response)
	// writeToServer(connection, "PASS 7SayEbTMvZkfVuQBeXCd\r\n")
}

// Read the response from the given server connection.
func readFromServer(connection net.Conn) (string, error) {

	// initialize a new Reader so ReadString() method can be used
	reader := bufio.NewReader(connection)

	// read response until a newline char is found (end of message
	// according to our protocol)
	line, readError := reader.ReadString('\n')

	// ensure no errors occurred during reading
	checkError(readError)

	// chop off the newline at the end of line (from the docs:
	// "returning a string containing the data up to and including the delimiter)"
	readLine := line[:len(line)-1]
	return readLine, nil
}

func readArgs() string {
	if len(os.Args) < 3 {
		panic("Please input a valid operation followed by one or two locations.")
	}
	operation, param1 := os.Args[1], os.Args[2]
	var param2 string

	if len(os.Args) == 4 {
		param2 = os.Args[3]
	}

	var valid bool = validAddresses(param1, param2)
	var username string
	var password string
	if (valid) {
		interpretParam(param1)
		interpretParam(param2)
	}

	return operation
}

func interpretParam(param string) (string, string) {
	var username string
	var password string

	if isRemote(param) {
		if !isValidURL(param) {
			panic("Invalid Remote file format!")
		}
		userSeparate := strings.Split(param[7:], "@")
		user, url := userSeparate[0], userSeparate[1]
		if strings.Contains(url, ":") {
			port = strings.Split(url, ":")[1]
		}
		hostname = strings.Split(url, ":")[0]

		if len(strings.Split(user, ":")) == 2 {
			username, password = strings.Split(user, ":")[0], strings.Split(user, ":")[1]
		} else {
			username, password = "anonymous", ""
		}
		return username, password
	}
	interpretLocalParam

}

func validAddresses(location1 string, location2 string) bool {
	return isRemote(location1) != isRemote(location2)
}

func isRemote(location string) bool {
	return strings.HasPrefix(location, "ftps://")
}

func isValidURL(url string) bool {
	if !isRemote(url) {
		return false
	}
	userSeparate := strings.Split(url[7:], "@")
	user := userSeparate[0]
	return len(userSeparate) == 2 && strings.Contains(user, ":")
}

// len != 2 || validURL == validURL -> throw error

func makeConn(connection string) (net.Conn, error) {
	return net.Dial("tcp", connection)
}

// Write the given data message to the given server connection.
func writeToServer(connection net.Conn, data string) {
	_, writeError := connection.Write([]byte(data))
	checkError(writeError)
}

// Checks if the given error exists and panics if it does. Else, do
// nothing since no error occurred.
func checkError(err error) {
	if err != nil {
		panic("failed in communication with server; reason: " + err.Error())
	}
}
