package main

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"io/ioutil"
	"golang.org/x/crypto/ssh"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: %s <user> <host:port>", os.Args[0])
	}

// 	key, err := getKeyFile()
// 	if err !=nil {
// 		panic(err)
//    }

// 	client, session, err := connectToHost(os.Args[1], os.Args[2], key)
// 	if err != nil {
// 		panic(err)
// 	}

	// var b bytes.Buffer
	// session.Stdout = &b
	// if err := session.Run("sudo echo hola"); err != nil {
	// 	panic("Failed to run: " + err.Error())
	// }
	// fmt.Println(b.String())
	// out, err := session.CombinedOutput("echo hola")
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(string(out))
	// client.Close()
	runStartupScript("centos", "192.168.100.17", "22", "lustre-ssh.key")
}

func runStartupScript(user string, hostIP string, port string, keyFileName string){
	key, err := getKeyFile(keyFileName)
	if err !=nil {
		panic(err)
   }

   var host string = hostIP + ":" + port
   client, session, err := connectToHost(user, host, key)
	if err != nil {
		panic(err)
	}

	var s string = "export S_PUB_KEY=\"" + keyFileName + "\""
	fmt.Println(s)
	// out, err := session.CombinedOutput(s)
	err = session.Setenv("SS_PUB_KEY", keyFileName)
	if err != nil {
		panic(err)
	}
	// fmt.Println(string(out))
	client.Close()
}

func getKeyFile(keyFileName string) (key ssh.Signer, err error){
    usr, _ := user.Current()
    file := usr.HomeDir + "/.ssh/" + keyFileName
	buf, err := ioutil.ReadFile(file)
    if err != nil {
        return
    }
	key, err = ssh.ParsePrivateKey(buf)
    if err != nil {
        return
     }
    return
}

func connectToHost(user, host string, key ssh.Signer) (*ssh.Client, *ssh.Session, error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, session, nil
}