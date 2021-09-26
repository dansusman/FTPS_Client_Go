package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

var hostname string

func main() {
	server, username, password, command, locations := readArgs()
	hostname = strings.Split(server, ":")[0]
	var filePath string
	if !strings.Contains(server, "/") {
		filePath = "/"
	} else {
		filePath = server[strings.Index(server, "/"):]
	}
	port := strings.Split(strings.Split(server, ":")[1], "/")[0]
	connection, err := makeConn(hostname + ":" + port)
	checkError(err)
	response := readFromServer(connection)
	fmt.Println(response)
	defer connection.Close()
	connection = startUp(connection, hostname, username, password)
	handleCommand(connection, command, locations, filePath)
}

func handleCommand(connection net.Conn, command string, locations []string, remoteFilePath string) {
	switch(command) {
	case "rmdir", "mkdir", "rm":
		writeToServer(connection, ftpCommand(command, locations, remoteFilePath))
		response := readFromServer(connection)
		fmt.Println(response)
	case "ls":
		list(connection, remoteFilePath)
	case "cp":
		copyFile(connection, locations)
	case "mv":
		moveFile(connection, locations)
	default:
		panic("Invalid command!")
	}
}

func copyFile(conn net.Conn, locations []string) {
	if isRemote(locations[0]) {
		retrieveFile(conn, locations[0], locations[1])
		writeToServer(conn, ftpCommand("rm", locations, locations[0]))
		response := readFromServer(conn)
		fmt.Println(response)
	} else {
		storeFile(conn, locations[0], locations[1])
		os.Remove(locations[0])
	}
}

func moveFile(conn net.Conn, locations []string) {
	if isRemote(locations[0]) {
		retrieveFile(conn, locations[0], locations[1])
		
	} else {
		storeFile(conn, locations[0], locations[1])
	}
	
}


func ftpCommand(command string, locations []string, remoteFilePath string) string {
	switch command {
	case "rm":
		return fmt.Sprintf("DELE %s\r\n", remoteFilePath)
	case "rmdir":
		return fmt.Sprintf("RMD %s\r\n", remoteFilePath)
	case "mkdir":
		return fmt.Sprintf("MKD %s\r\n", remoteFilePath)
	default:
		panic("Invalid command!")
	}
}

func initializeDataChannel(connection net.Conn, command string) net.Conn {
	writeToServer(connection, "PASV\r\n")
	response := readFromServer(connection)
	hostIp, port := verifyDataChannelResponse(response)
	fmt.Println(response)
	writeToServer(connection, command)
	return startDataChannel(hostIp, port)
}

func list(connection net.Conn, filePath string) string {
	// 1. Send the PASV command
	// 2. Write command
	// 3. Open new socket for data channel and connect to IP and port from step 1
	dataChannel := initializeDataChannel(connection, fmt.Sprintf("LIST %s\r\n", filePath))
	// 4. Read server's response to the command in step 2
	response := readFromServer(connection)
	fmt.Println(response)
	// If the response contains an error code, close the data channel and abort.
	responseCode := strings.Split(response, " ")[0]
	if responseCode[0] == '4' || responseCode[0] == '5' || responseCode[0] == '6' {
		dataChannel.Close()
		panic("Error in control command: " + response)
	}
	// 5. Wrap data channel socket in TLS
	dataChannel = tls.Client(dataChannel, &tls.Config{ServerName: hostname})
	// 6. Receive data on data channel
	reader := bufio.NewReader(dataChannel)
	for {
		line, readErr := reader.ReadString('\n')
		if (readErr == io.EOF) {
			break;
		}
		readLine := line[:len(line)-1]
		fmt.Println(readLine)
	}
	// 7. Close the data channel.
	dataChannel.Close()
	// 8. Read the final response from the server on the control channel.
	response = readFromServer(connection)
	fmt.Println(response)
	return response
}

func retrieveFile(conn net.Conn, remoteFilePath string, localFilePath string) string {
	initUploadDownload(conn)
	dataChannel := initializeDataChannel(conn, fmt.Sprintf("RETR %s\r\n", remoteFilePath))
	response := readFromServer(conn)
	fmt.Println(response)
	responseCode := strings.Split(response, " ")[0]
	if responseCode[0] == '4' || responseCode[0] == '5' || responseCode[0] == '6' {
		dataChannel.Close()
		panic("Error in control command: " + response)
	}
	defer dataChannel.Close()
	dataChannel = tls.Client(dataChannel, &tls.Config{ServerName: hostname})

	file, fileErr := os.Create(localFilePath)
	if fileErr != nil {
		panic("File error: " + fileErr.Error())
	}

	defer file.Close()

	_, copyErr := io.Copy(file, conn)
	checkError(copyErr)

	dataChannel.Close()
	response = readFromServer(conn)
	fmt.Println(response)
	return response
}

func storeFile(conn net.Conn, remoteFilePath string, localFilePath string) string {
	initUploadDownload(conn)
	dataChannel := initializeDataChannel(conn, fmt.Sprintf("STOR %s\r\n", remoteFilePath))
	response := readFromServer(conn)
	fmt.Println(response)
	responseCode := strings.Split(response, " ")[0]
	if responseCode[0] == '4' || responseCode[0] == '5' || responseCode[0] == '6' {
		dataChannel.Close()
		panic("Error in control command: " + response)
	}
	defer dataChannel.Close()
	dataChannel = tls.Client(dataChannel, &tls.Config{ServerName: hostname})
	return ""

}

func initUploadDownload(conn net.Conn) {
	writeToServer(conn, "TYPE I\r\n")
	response := readFromServer(conn)
	fmt.Println(response)
	writeToServer(conn, "MODE S\r\n")
	response = readFromServer(conn)
	fmt.Println(response)
	writeToServer(conn, "STRU F\r\n")
	response = readFromServer(conn)
	fmt.Println(response)
}

func startDataChannel(hostIp string, port string) net.Conn {
	dataChannel, err := makeConn(hostIp + ":" + port)
	checkError(err)
	return dataChannel
}

func verifyDataChannelResponse(response string) (string, string) {
	responseSplit := strings.Split(response, " ")
	if responseSplit[0] != strconv.Itoa(227) || len(responseSplit) != 5 {
		panic("Failed to establish data channel!")
	}
	ipAddressWithPort := strings.Split(responseSplit[4][1:len(responseSplit[4])-3], ",")
	ipAddress := ipAddressWithPort[:4]
	portStart, convertErr := strconv.Atoi(ipAddressWithPort[4])
	checkError(convertErr)
	portEnd, convertErr := strconv.Atoi(ipAddressWithPort[5])
	checkError(convertErr)
	port := (portStart << 8) + portEnd
	return strings.Join(ipAddress, "."), strconv.Itoa(port)
}

func authTLS(conn net.Conn) net.Conn {
	writeToServer(conn, "AUTH TLS\r\n")
	response := readFromServer(conn)
	fmt.Println(response)
	conn = tls.Client(conn, &tls.Config{ServerName: hostname})
	return conn
}

func startUp(conn net.Conn, hostname string, username string, password string) net.Conn {
	conn = authTLS(conn)
	writeToServer(conn, fmt.Sprintf("USER %s\r\n", username))
	response := readFromServer(conn)
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
	return conn
}

// Read the response from the given server connection.
func readFromServer(connection net.Conn) (string) {
	reader := bufio.NewReader(connection)
	line, readError := reader.ReadString('\n')
	checkError(readError)
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
	// if len(strings.Split(url, "/")) < 2 {
	// 	panic("Must include file path in remote URL!")
	// }
	urlPieces := strings.Split(url, ".")
	if len(urlPieces) < 2 {
		panic("Invalid remote URL format!")
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
		panic("Error: " + err.Error())
	}
}
