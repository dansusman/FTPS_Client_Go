package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	server, username, password, command, locations := readArgs()
	hostname, port, filePath := strings.Split(server, ":")[0], strings.Split(strings.Split(server, ":")[1], "/")[0], server[strings.Index(server, "/"):]
	fmt.Println(filePath, command, locations)
	connection, err := makeConn(hostname + ":" + port)
	checkError(err)
	response := readFromServer(connection)
	fmt.Println(response)
	defer connection.Close()
	startUp(connection, hostname, username, password)
	handleCommand(connection, command, locations, filePath)
}

func handleCommand(connection net.Conn, command string, locations []string, remoteFilePath string) {
	fmt.Println(ftpCommand(command, locations, remoteFilePath))
	writeToServer(connection, ftpCommand(command, locations, remoteFilePath))
	response := readFromServer(connection)
	fmt.Println(response)
}

func ftpCommand(command string, locations []string, remoteFilePath string) string {
	switch command {
	case "ls":
		return fmt.Sprintf("LIST %s\r\n", remoteFilePath)
	case "rm":
		return fmt.Sprintf("DELE %s\r\n", remoteFilePath)
	case "rmdir":
		return fmt.Sprintf("RMD %s\r\n", remoteFilePath)
	case "mkdir":
		return fmt.Sprintf("MKD %s\r\n", remoteFilePath)
	// case "cp": return fmt.Sprintf("")
	// case "mv":
	default:
		panic("Invalid command!")
	}
}

func startUp(conn net.Conn, hostname string, username string, password string) {
	writeToServer(conn, "AUTH TLS\r\n")
	response := readFromServer(conn)
	fmt.Println(response)
	conn = tls.Client(conn, &tls.Config{ServerName: hostname})
	writeToServer(conn, fmt.Sprintf("USER %s\r\n", username))
	response = readFromServer(conn)
	fmt.Println(response)
	writeToServer(conn, fmt.Sprintf("PASS %s\r\n", password))
	response = readFromServer(conn)
	fmt.Println(response)
	writeToServer(conn, "PBSZ 0\r\n")
	response = readFromServer(conn)
	fmt.Println(response)
	writeToServer(conn, "PROT P\r\n")
	response = readFromServer(conn)
	fmt.Println(response)
	writeToServer(conn, "TYPE I\r\n")
	response = readFromServer(conn)
	fmt.Println(response)
	writeToServer(conn, "MODE S\r\n")
	response = readFromServer(conn)
	fmt.Println(response)
	writeToServer(conn, "STRU F\r\n")
	response = readFromServer(conn)
	fmt.Println(response)
}

// Read the response from the given server connection.
func readFromServer(connection net.Conn) string {

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
	return readLine
}

func readArgs() (string, string, string, string, []string) {
	if len(os.Args) < 3 || !validCommand(os.Args[1], os.Args[2:]) {
		panic("Please input a valid FTP operation followed by one or two locations.")
	}
	var remoteURL string
	if isRemote(os.Args[2]) {
		remoteURL = os.Args[2]
	} else {
		remoteURL = os.Args[3]
	}
	command, locations := os.Args[1], os.Args[2:]
	server, username, password := parseRemote(remoteURL)

	return server, username, password, command, locations
}

func parseRemote(remoteURL string) (string, string, string) {
	userSeparate := strings.Split(remoteURL[7:], "@")
	user, url := userSeparate[0], userSeparate[1]
	username, password := strings.Split(user, ":")[0], strings.Split(user, ":")[1]
	return url, username, password
}

func validCommand(command string, params []string) bool {
	if command == "ls" || command == "rm" || command == "rmdir" || command == "mkdir" {
		return len(params) == 1 && validRemoteLocation(params[0])
	} else if command == "cp" || command == "mv" {
		return len(params) == 2 && validAddresses(params[0], params[1])
	} else {
		return false
	}
}

func validRemoteLocation(location string) bool {
	if !strings.HasPrefix(location, "ftps://") {
		panic("Please input a valid URL beginning with ftps://")
	}

	userSeparate := strings.Split(location[7:], "@")

	if len(userSeparate) != 2 {
		panic("Please supply URL in format ftps://<username>:<password>@<location.com>/<path-to-file>!")
	}

	user, url := userSeparate[0], userSeparate[1]

	if len(strings.Split(user, ":")) != 2 {
		panic("Please include your login username and password in your FTP URL.")
	}

	urlPieces := strings.Split(url, ".")

	if len(urlPieces) < 2 {
		panic("Invalid remote URL format!")
	}

	if len(strings.Split(urlPieces[len(urlPieces)-1], "/")) < 2 {
		panic("Must include file path in remote URL!")
	}

	return true
}

func validAddresses(location1 string, location2 string) bool {
	return (isRemote(location1) && validRemoteLocation(location1)) !=
		(isRemote(location2) && validRemoteLocation(location2))
}

func isRemote(location string) bool {
	return strings.HasPrefix(location, "ftps://")
}

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
