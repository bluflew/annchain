// Copyright 2017 Annchain Information Technology Services Co.,Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	agtypes "github.com/annchain/annchain/angine/types"
	"github.com/annchain/annchain/client/commons"
	ac "github.com/annchain/annchain/module/lib/go-common"
	cl "github.com/annchain/annchain/module/lib/go-rpc/client"
	"github.com/annchain/annchain/types"
	"github.com/annchain/anth/rlp"
	homedir "github.com/mitchellh/go-homedir"
	"go.uber.org/zap"
)

/*
  请将sender address 做为init_token，初始化给予一定balance
*/
var (
	senderpriv    = "1956be6c7e5128bd1c44c389ba21bd63cfb054d5adb2ab3f8d968f74b9bd0b6b"
	senderpub     = "04e329956f162146cad0d07ddecb5f95329542b1b3badf5b4fe507fd6f0556118326d251e051bdc080bd0f50c0b94d054bcd9b25a2177003296e76a8c07d9b6e22"
	senderaddress = "addafebb1c4618f8f8b452dab6d53721f1d9fda6"

	receiver    = "C7038C9F5FDE83EB3A6341EA8AC95D05BCB3BBAB"
	logger      *zap.Logger
	anntoolPath = ""
	annPath     = ""
	nodeChan    = make(chan *exec.Cmd, 1)
	runtimePath = ""
)

func init() {
	runtimePath, _ = homedir.Dir()
	runtimePath = path.Join(runtimePath, ".angine")
	var err error
	anntoolPath, err = exec.LookPath("../../build/anntool")
	if err != nil {
		fmt.Println("cannot find executable file anntool:", err)
		os.Exit(-1)
	}
	annPath, err = exec.LookPath("../../build/ann")
	if err != nil {
		fmt.Println("cannot find executable file ann:", err)
		os.Exit(-1)
	}
	//run node
	cmd := exec.Command(annPath, []string{"run"}...)
	go func() {
		cmd.Run()
	}()
	nodeChan <- cmd
	time.Sleep(time.Second * 2)
}

//test evm transaction
func TestTransfer(t *testing.T) {
	msg := make(chan bool)
	nonce, err := getNonce(senderaddress)
	if err != nil {
		t.Error(err)
	}
	go func() {
		args := []string{"tx", "send", "--privkey", senderpriv, "--to", receiver, "--value", "999", "--nonce", strconv.FormatUint(nonce, 10)}
		_, err := exec.Command(anntoolPath, args...).Output()
		if err != nil {
			t.Error(err)
		}
		close(msg)
	}()
	<-msg
	time.Sleep(time.Second * 1)
	args := []string{"query", "balance", "--address", receiver}
	outs, err := exec.Command(anntoolPath, args...).Output()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(receiver, " balance :", string(outs))
}

//test contract create
func TestContractCreate(t *testing.T) {
	msg := make(chan []byte, 256)
	defer close(msg)
	nonce, err := getNonce(senderaddress)
	go func() {
		args := []string{"evm", "create", "--callf", "./contract/sample.json", "--abif", "./contract/sample.abi", "--nonce", strconv.FormatUint(nonce, 10)}
		hashbytes, err := exec.Command(anntoolPath, args...).Output()
		if err != nil {
			t.Error(err)
		}
		msg <- hashbytes
	}()
	hash := <-msg
	ss := strings.Split(string(hash), "contract")
	txhash := strings.TrimSpace(strings.TrimRight(strings.TrimLeft(string(ss[0]), "txHash:"), "contract address"))
	time.Sleep(time.Second * 1)
	args := []string{"query", "receipt", "--hash", txhash}
	outs, err := exec.Command(anntoolPath, args...).Output()
	if err != nil {
		t.Error(err)
	}
	fmt.Println("receipt :", string(outs))
}

//test contract exit
func TestContractExist(t *testing.T) {
	args := []string{"evm", "exist", "--callf", "./contract/sample_exist.json"}
	outs, err := exec.Command(anntoolPath, args...).Output()
	if err != nil {
		t.Error(err)
	}
	fmt.Println("exist ? ", string(outs))
}

//test contract exec
func TestContractExec(t *testing.T) {
	msg := make(chan []byte, 256)
	defer close(msg)
	nonce, err := getNonce(senderaddress)
	go func() {
		args := []string{"evm", "execute", "--callf", "./contract/sample_execute.json", "--abif", "./contract/sample.abi", "--nonce", strconv.FormatUint(nonce, 10)}
		hashbytes, err := exec.Command(anntoolPath, args...).Output()
		if err != nil {
			t.Error(err)
		}
		msg <- hashbytes
	}()
	hash := <-msg
	txhash := strings.TrimSpace(strings.TrimLeft(string(hash), "txHash:"))
	time.Sleep(time.Second * 1)
	args := []string{"query", "receipt", "--hash", txhash}
	outs, err := exec.Command(anntoolPath, args...).Output()
	if err != nil {
		t.Error(err)
	}
	fmt.Println("receipt :", string(outs))
}

//test contract read
func TestContractRead(t *testing.T) {
	nonce, err := getNonce(senderaddress)
	args := []string{"evm", "read", "--callf", "./contract/sample_read.json", "--abif", "./contract/sample.abi", "--nonce", strconv.FormatUint(nonce, 10)}
	outs, err := exec.Command(anntoolPath, args...).Output()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(string(outs))
}

func getNonce(addr string) (nonce uint64, err error) {
	clientJSON := cl.NewClientJSONRPC(logger, commons.QueryServer)
	res := new(agtypes.ResultQuery)

	addrHex := ac.SanitizeHex(addr)
	adr, _ := hex.DecodeString(addrHex)
	query := append([]byte{types.QueryTypeNonce}, adr...)

	_, err = clientJSON.Call("query", []interface{}{query}, res)
	if err != nil {
		return 0, err
	}

	//nonce = binary.LittleEndian.Uint64(res.Result.Data)
	rlp.DecodeBytes(res.Result.Data, &nonce)
	return nonce, nil
}
