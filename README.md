# Golang FTPS Client

This repo holds the source code for a simple FTPS Client written in Go. This was project 2 for my Networks and Distributed Systems course at NEU.

## High Level Approach

After reading the rather lengthy project description a few times over, I decided following the [suggested implementation approach](https://3700.network/docs/projects/ftpclient/#suggested-implementation-approach) section from the docs was likely my best bet at effectively and efficiently implementing this FTPS client. I worked from the first bullet down and things went smoothly for the most part.

Overall, my code is put together simply. The command line arguments are parsing into a useable format so the username, password, sever name/port, and remote/local file paths are separated. The command is validated for correct form, i.e. if the command requires two arguments, two valid arguments are supplied and if only one argument is needed that argument is supplied and valid. These two steps make use of the [os Go package](https://pkg.go.dev/os).

The next major part of the code is initializing the correct sockets at the correct time. I started with just rm, rmdir, and mkdir since they do not require a data channel. I utilize the [net Go package](https://pkg.go.dev/net) for this. The net package allows one to connect with a non-TLS encrypted socket. With an unencrypted socket set up, my program can then send "AUTH TLS" through the control channel and successfully read a response. From there, I utilize the [tls Go package](https://pkg.go.dev/crypto/tls) to wrap the existing control socket in TLS encryption, as described in the spec. 

With this set up, I followed the specified steps required to upload/download data. For the most part, this was simple since the sequence of steps was similar to project 1: write to a server, read response, do something with the response.

I encapsulated the more complex operations in separate functions (see copyFile, moveFile, and list functions), and rmdir, mkdir, and rm were simple and similar enough that I created a small helper function ftpCommand that translates a Linux terminal command to FTP format.

Copy and move utilize the os package to create/open local files and the [io Go package](https://pkg.go.dev/io) to copy file contents to and from the FTPS server. Both functionalities are just clever uses of retrieve and store (RETR and STOR) based on whether the source and destination are local and remote, respectively, or vice versa.

## Challenges

I am much more familiar with Go now than project 1, so the main challenges I faced were on a slightly different plane. This project brought some very complex parsing challenges, so getting the validation correct was a bit painstaking. Also, getting the order of FTPS commands exactly correct for each Linux command was a challenge. Luckily, the docs were very clear and Piazza quite active. A third challenge was debugging some small errors like hanging code because of open data channel, and 5xx codes because of using the control channel when I meant to use the data channel. Those were frustrating but rewarding to fix.

Otherwise, the project was enjoyable and straightforward. :)

## Testing Code

To test my code, I placed print statements before each write to the server and after each read from the server. This allowed me to catch any bugs in my parsing and differentiate conceptual errors (e.g. out of order code) from small careless errors (e.g. incorrect string splitting). 

Once the functionality was complete, I tested that each function worked exactly as expected by visually checking the contents on the server. I used [Filestash](https://demo.filestash.app/) to do this. This website allows you to log into an FTP server and view/update what's there.

The process looked something like this:

    1. Call "ls" to see what's there
    2. Open Filestash and compare to "ls"
    3. Call "mkdir"
    4. Open Filestash to verify the directory was added
    5. Call "mv" <local file> <remote path>
    6a. Open Filestash to verify the directory is there
    6b. Call "ls" to check the update is present
    7. Call "cp" <remote file> <local path>
    8. Check the file contents match what is expected
    9. Call "rmdir"
    5. Open Filestash/Call "ls" to verify the directory is gone
