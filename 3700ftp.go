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

// hostname of FTP server
var hostname string

// Main function that runs the FTPS client.
// Reads inputs from the command line, parsing for important
// information such as hostname, port, file paths, and desired command.
// Creates a connection with server hostname:port, runs start up, then
// handles the input command using the input source and/or destination.
func main() {
	// parse and verify command line arguments
	server, username, password, command, locations := readArgs()
	hostname = strings.Split(server, ":")[0]
	var filePath string
	if !strings.Contains(server, "/") {
		filePath = "/"
	} else {
		filePath = server[strings.Index(server, "/"):]
	}
	port := strings.Split(strings.Split(server, ":")[1], "/")[0]
	// create connection with server
	connection, err := makeConn(hostname + ":" + port)
	checkError(err)
	response := readFromServer(connection)
	fmt.Println(response)
	defer connection.Close()
	// run start up
	connection = startUp(connection, hostname, username, password)
	// perform the command as inputted through command line
	handleCommand(connection, command, locations, filePath)
}

// Performs the command as specified by the arg "command". Currently,
// this client supports rmdir, mkdir, rm, ls, cp, and mv.
func handleCommand(connection net.Conn, command string, locations []string, remoteFilePath string) {
	switch command {
	case "rmdir", "mkdir", "rm":
		// if the command is rmdir, mkdir, or rm,
		// we just write to control socket and grab response
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
		// incorrect command slipped through; this case should never be reached
		panic("Invalid command!")
	}
}

// Performs the "cp" command through FTPS. This copys a file from
// source location to destination location (the two strings in arg "locations").
func copyFile(conn net.Conn, locations []string) {
	if isRemote(locations[0]) {
		// if the first location is remote,
		// we know we want to RETR from server, so
		// parse remote for the file path and call RETR
		url, _, _ := parseRemote(locations[0])
		filePath := url[strings.Index(url, "/"):]
		retrieveFile(conn, filePath, locations[1])
	} else {
		// if the first location is not remote,
		// we know the second location must be
		// (as checked by validAddresses function).
		// we also know that we want to STOR to server
		// so parse remote file path and call STOR
		url, _, _ := parseRemote(locations[1])
		filePath := url[strings.Index(url, "/"):]
		storeFile(conn, filePath, locations[0])
	}
}

// Performs the "mv" command through FTPS. Similar to copyFile,
// but removes the source file as well.
func moveFile(conn net.Conn, locations []string) {
	if isRemote(locations[0]) {
		// if the first location is remote,
		// call RETR, then DELE
		url, _, _ := parseRemote(locations[0])
		filePath := url[strings.Index(url, "/"):]
		fmt.Println(filePath)
		retrieveFile(conn, filePath, locations[1])
		handleCommand(conn, "rm", locations, filePath)
	} else {
		// otherwise, the second location is remote,
		// so call STOR and then delete the local file
		// (source file must be local, as guaranteed by verifyCommand)
		url, _, _ := parseRemote(locations[1])
		filePath := url[strings.Index(url, "/"):]
		fmt.Println(filePath)
		storeFile(conn, filePath, locations[0])
		os.Remove(locations[0])
	}
}

// Converts the given arg "command" into a valid FTP formatted command.
// E.g. "rm file.txt" becomes "DELE file.txt\r\n"
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

// Initializes the data channel socket. Returns the data channel connection object.
func initializeDataChannel(connection net.Conn, command string) net.Conn {
	// Ask the FTPS server to open a data channel.
	writeToServer(connection, "PASV\r\n")
	response := readFromServer(connection)
	// validate response from PASV and parse for IP address and port
	hostIp, port := verifyDataChannelResponse(response)
	fmt.Println(response)
	writeToServer(connection, command)
	return startDataChannel(hostIp, port)
}

// Performs the "ls" command through FTPS. This lists the file
// information of the specified filePath.
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
	// 6. Receive data on data channel. Since the response may be
	// longer than a single line, read until EOF is reached. This
	// guarantees that all the ls information is received and printed.
	reader := bufio.NewReader(dataChannel)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr == io.EOF {
			break
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

// Retrieves a file as specified by remoteFilePath from the FTPS server,
// and places it at localFilePath on the client's machine.
func retrieveFile(conn net.Conn, remoteFilePath string, localFilePath string) string {
	// prep the server for Upload/Download
	initUploadDownload(conn)
	// initialize the data channel and call RETR to control socket
	dataChannel := initializeDataChannel(conn, fmt.Sprintf("RETR %s\r\n", remoteFilePath))
	response := readFromServer(conn)
	fmt.Println(response)
	// verify that the data channel response is not an error
	responseCode := strings.Split(response, " ")[0]
	if responseCode[0] == '4' || responseCode[0] == '5' || responseCode[0] == '6' {
		dataChannel.Close()
		panic("Error in control command, retrieving: " + response)
	}

	// wrap data channel in TLS
	dataChannel = tls.Client(dataChannel, &tls.Config{ServerName: hostname})
	defer dataChannel.Close()

	// create a file on client's machine at localFilePath
	file, fileErr := os.Create(localFilePath)
	defer file.Close()
	if fileErr != nil {
		panic("File error: " + fileErr.Error())
	}
	// copy the information from the server to the newly created file
	_, copyErr := io.Copy(file, dataChannel)
	checkError(copyErr)

	// close data channel socket
	dataChannel.Close()
	response = readFromServer(conn)
	fmt.Println(response)
	return response
}

// Stores a file from the client's local machine at remoteFilePath in the FTPS server.
func storeFile(conn net.Conn, remoteFilePath string, localFilePath string) string {
	// prep control socket for Upload/Download
	initUploadDownload(conn)
	// set up data channel and call STOR to control socket
	dataChannel := initializeDataChannel(conn, fmt.Sprintf("STOR %s\r\n", remoteFilePath))
	response := readFromServer(conn)
	fmt.Println(response)
	// verify response is not an error
	responseCode := strings.Split(response, " ")[0]
	if responseCode[0] == '4' || responseCode[0] == '5' || responseCode[0] == '6' {
		dataChannel.Close()
		panic("Error in control command, storing: " + response)
	}
	// wrap data channel socket in TLS
	dataChannel = tls.Client(dataChannel, &tls.Config{ServerName: hostname})
	defer dataChannel.Close()

	// open the local file we are going to store
	file, openErr := os.Open(localFilePath)
	defer file.Close()
	if openErr != nil {
		panic("File error: " + openErr.Error())
	}

	// copy the data from the open local file to the data channel
	_, copyErr := io.Copy(dataChannel, file)
	checkError(copyErr)

	// close the data channel
	dataChannel.Close()
	response = readFromServer(conn)
	fmt.Println(response)
	return response
}

// Preps the control socket for upload/download of data.
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

// Creates a connection to the data channel, as specified by
// the hostIp IP Address and the port number.
func startDataChannel(hostIp string, port string) net.Conn {
	dataChannel, err := makeConn(hostIp + ":" + port)
	checkError(err)
	return dataChannel
}

// Verifies that the data channel response from PASV is of valid form
// and parses for the IP Address and Port of the data channel.
// Returns the IP address and port number of the data channel.
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

// Informs the control socket that TLS is required,
// and wraps the given connection object in TLS.
// Returns the given socket wrapped in TLS.
func authTLS(conn net.Conn) net.Conn {
	writeToServer(conn, "AUTH TLS\r\n")
	response := readFromServer(conn)
	fmt.Println(response)
	conn = tls.Client(conn, &tls.Config{ServerName: hostname})
	return conn
}

// Preps the control socket with username, password, TLS encryption, private
// protection level, and a protection buffer of zero bytes. These
// steps must be run prior to any FTPS commands.
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
func readFromServer(connection net.Conn) string {
	reader := bufio.NewReader(connection)
	line, readError := reader.ReadString('\n')
	checkError(readError)
	readLine := line[:len(line)-1]
	return readLine
}

// Reads the command line arguments, verifies their validity/format matches
// what is expected, and parses for the server name, username, password,
// command, and source/destination location(s).
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

// Parses a remote URL into url, username, and password sections.\
// Valid Remote URLs are of the form ftps://<username>:<password>@<location.com>/<path-to-file>.
func parseRemote(remoteURL string) (string, string, string) {
	userSeparate := strings.Split(remoteURL[7:], "@")
	user, url := userSeparate[0], userSeparate[1]
	username, password := strings.Split(user, ":")[0], strings.Split(user, ":")[1]
	return url, username, password
}

// Returns true if the given command is valid, i.e. is one of the
// six supported commands, the number of arguments to the command
// is valid, and the arguments are of the proper type (remote vs local).
func validCommand(command string, params []string) bool {
	if command == "ls" || command == "rm" || command == "rmdir" || command == "mkdir" {
		// if the command is "ls", "rm", "rmdir", or "mkdir",
		// we expect exactly one argument. Said argument
		// must be a valid remote URL
		return len(params) == 1 && validRemoteLocation(params[0])
	} else if command == "cp" || command == "mv" {
		// if the command is "cp" or "mv", we expect
		// exactly two arguments. Said arguments
		// must be a valid remote URL and a local path.
		return len(params) == 2 && validAddresses(params[0], params[1])
	} else {
		// otherwise, invalid command, so return false
		return false
	}
}

// Returns true iff the given URL is a valid remote URL, i.e.
// it has the prefix "ftps://", it contains a username and password,
// and it contains a remote URL (e.g. ftp.3700.network) of the correct form.
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
	return true
}

// Returns true if the two given file paths are valid, i.e. one is
// a valid remote URL and the other is not remote.
func validAddresses(location1 string, location2 string) bool {
	return (isRemote(location1) && validRemoteLocation(location1)) !=
		(isRemote(location2) && validRemoteLocation(location2))
}

// Returns true if the given file path begins with "ftps://".
func isRemote(location string) bool {
	return strings.HasPrefix(location, "ftps://")
}

// Creates a connection to the given server name.
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
