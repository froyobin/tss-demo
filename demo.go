package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/tss/go-tss/tss"
	moneroTss "gitlab.com/thorchain/tss/monero-tss/tss"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	btsskeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	maddr "github.com/multiformats/go-multiaddr"
	"gitlab.com/thorchain/tss/go-tss/common"
	"gitlab.com/thorchain/tss/go-tss/conversion"
	"gitlab.com/thorchain/tss/go-tss/keygen"
	moneroCommon "gitlab.com/thorchain/tss/monero-tss/common"
	moneroKeygen "gitlab.com/thorchain/tss/monero-tss/monero_multi_sig/keygen"
)

const (
	partyNum         = 4
	testFileLocation = "./test_data"
	preParamTestFile = "preParam_test.data"
)

var (
	testPubKeys = []string{
		"thorpub1addwnpepqtdklw8tf3anjz7nn5fly3uvq2e67w2apn560s4smmrt9e3x52nt2svmmu3",
		"thorpub1addwnpepqtspqyy6gk22u37ztra4hq3hdakc0w0k60sfy849mlml2vrpfr0wvm6uz09",
		"thorpub1addwnpepq2ryyje5zr09lq7gqptjwnxqsy2vcdngvwd6z7yt5yjcnyj8c8cn559xe69",
		"thorpub1addwnpepqfjcw5l4ay5t00c32mmlky7qrppepxzdlkcwfs2fd5u73qrwna0vzag3y4j",
	}
	testPriKeyArr = []string{
		"MjQ1MDc2MmM4MjU5YjRhZjhhNmFjMmI0ZDBkNzBkOGE1ZTBmNDQ5NGI4NzM4OTYyM2E3MmI0OWMzNmE1ODZhNw==",
		"YmNiMzA2ODU1NWNjMzk3NDE1OWMwMTM3MDU0NTNjN2YwMzYzZmVhZDE5NmU3NzRhOTMwOWIxN2QyZTQ0MzdkNg==",
		"ZThiMDAxOTk2MDc4ODk3YWE0YThlMjdkMWY0NjA1MTAwZDgyNDkyYzdhNmMwZWQ3MDBhMWIyMjNmNGMzYjVhYg==",
		"ZTc2ZjI5OTIwOGVlMDk2N2M3Yzc1MjYyODQ0OGUyMjE3NGJiOGRmNGQyZmVmODg0NzQwNmUzYTk1YmQyODlmNA==",
	}
)

type FourDemoSuite struct {
	servers       []*tss.TssServer
	moneroServers []*moneroTss.TssServer
	ports         []int
	moneroPorts   []int
	preParams     []*btsskeygen.LocalPreParams
	bootstrapPeer string
	logger        zerolog.Logger
	rpcAddress    []string
}

// setup four nodes for test
func (s *FourDemoSuite) SetUpTest() error {
	var globalErr error
	common.InitLog("info", true, "four_nodes_test")
	s.logger = log.With().Str("demo", "tss").Logger()
	conversion.SetupBech32Prefix()
	s.ports = []int{16666, 16667, 16668, 16669}
	s.moneroPorts = []int{1777, 1778, 1779, 1780}
	s.bootstrapPeer = "/ip4/127.0.0.1/tcp/16666/p2p/16Uiu2HAmACG5DtqmQsHtXg4G2sLS65ttv84e7MrL4kapkjfmhxAp"
	s.preParams = getPreparams(s.logger)
	s.servers = make([]*tss.TssServer, partyNum)
	s.moneroServers = make([]*moneroTss.TssServer, partyNum)
	s.rpcAddress = make([]string, partyNum)

	remoteAddress := []string{"134.209.108.57", "167.99.11.83", "46.101.91.4", "134.209.35.249", "174.138.10.57", "134.209.101.44"}
	for i := 0; i < partyNum; i++ {
		rpcAddress := fmt.Sprintf("http://%s:18083/json_rpc", remoteAddress[i])
		s.rpcAddress[i] = rpcAddress
	}

	conf := common.TssConfig{
		KeyGenTimeout:   90 * time.Second,
		KeySignTimeout:  90 * time.Second,
		PreParamTimeout: 5 * time.Second,
		EnableMonitor:   false,
	}

	moneroConf := moneroCommon.TssConfig{
		KeyGenTimeout:   90 * time.Second,
		KeySignTimeout:  90 * time.Second,
		PreParamTimeout: 5 * time.Second,
		EnableMonitor:   false,
	}
	_ = moneroConf

	var wg sync.WaitGroup
	for i := 0; i < partyNum; i++ {
		wg.Add(1)
		go func(idx int) {
			var err error
			defer wg.Done()
			if idx == 0 {
				s.servers[idx], err = s.getTssServer(idx, conf, "", "normal_tss")
				if err != nil {
					globalErr = err
				}
				//s.moneroServers[idx], err = s.getMoneroTssServer(idx, moneroConf, "", "monero_tss")
				//if err != nil {
				//	globalErr = err
				//}

			} else {
				s.servers[idx], err = s.getTssServer(idx, conf, s.bootstrapPeer, "normal_tss")
				if err != nil {
					globalErr = err
				}

				//s.moneroServers[idx], err = s.getMoneroTssServer(idx, moneroConf, s.bootstrapPeer, "monero_tss")
				//if err != nil {
				//	s.logger.Error().Err(err).Msgf("monero ")
				//	globalErr = err
				//}
			}
		}(i)

		time.Sleep(time.Second)
	}
	wg.Wait()
	for i := 0; i < partyNum; i++ {
		if err := s.servers[i].Start(); err != nil {
			return err
		}
	}
	return nil
}

func hash(payload []byte) []byte {
	h := sha256.New()
	h.Write(payload)
	return h.Sum(nil)
}

// generate a new key
func (s *FourDemoSuite) doTestKeygen(newJoinParty bool) {
	wg := sync.WaitGroup{}
	lock := &sync.Mutex{}
	keygenResult := make(map[int]keygen.Response)
	for i := 0; i < partyNum; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var req keygen.Request
			localPubKeys := append([]string{}, testPubKeys...)
			if newJoinParty {
				req = keygen.NewRequest(localPubKeys, 10, "0.14.0")
			} else {
				req = keygen.NewRequest(localPubKeys, 10, "0.13.0")
			}
			res, err := s.servers[idx].Keygen(req)
			if err != nil {
				s.logger.Error().Err(err).Msgf("fail to generate the key")
			}
			lock.Lock()
			defer lock.Unlock()
			keygenResult[idx] = res
		}(i)
	}
	wg.Wait()

	fmt.Printf("-------->pool address %v\n", keygenResult[0].PoolAddress)
	return
}

func (s *FourDemoSuite) TearDownTest() {
	// give a second before we shutdown the network
	time.Sleep(time.Second)
	for i := 0; i < partyNum; i++ {
		s.servers[i].Stop()
	}
	for i := 0; i < partyNum; i++ {
		tempFilePath := path.Join(os.TempDir(), "4nodes_test", strconv.Itoa(i))
		os.RemoveAll(tempFilePath)

	}
}

func (s *FourDemoSuite) getTssServer(index int, conf common.TssConfig, bootstrap, p2ptag string) (*tss.TssServer, error) {
	priKey, err := conversion.GetPriKey(testPriKeyArr[index])
	if err != nil {
		return nil, err
	}
	baseHome := path.Join(os.TempDir(), "4nodes_test", strconv.Itoa(index))
	if _, err := os.Stat(baseHome); os.IsNotExist(err) {
		err := os.MkdirAll(baseHome, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	var peerIDs []maddr.Multiaddr
	if len(bootstrap) > 0 {
		multiAddr, err := maddr.NewMultiaddr(bootstrap)
		if err != nil {
			return nil, err
		}
		peerIDs = []maddr.Multiaddr{multiAddr}
	} else {
		peerIDs = nil
	}
	instance, err := tss.NewTss(peerIDs, s.ports[index], priKey, p2ptag, baseHome, conf, s.preParams[index], "")
	return instance, err
}

func (s *FourDemoSuite) getMoneroTssServer(index int, conf moneroCommon.TssConfig, bootstrap, p2ptag string) (*moneroTss.TssServer, error) {
	priKey, err := conversion.GetPriKey(testPriKeyArr[index])
	if err != nil {
		return nil, err
	}
	baseHome := path.Join(os.TempDir(), "4nodes_test", strconv.Itoa(index))
	if _, err := os.Stat(baseHome); os.IsNotExist(err) {
		err := os.MkdirAll(baseHome, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	var peerIDs []maddr.Multiaddr
	if len(bootstrap) > 0 {
		multiAddr, err := maddr.NewMultiaddr(bootstrap)
		if err != nil {
			return nil, err
		}
		peerIDs = []maddr.Multiaddr{multiAddr}
	} else {
		peerIDs = nil
	}
	instance, err := moneroTss.NewTss(peerIDs, s.ports[index], priKey, p2ptag, baseHome, conf, s.preParams[index], "")
	return instance, err
}

func getPreparams(logger zerolog.Logger) []*btsskeygen.LocalPreParams {
	var preParamArray []*btsskeygen.LocalPreParams
	buf, err := ioutil.ReadFile(path.Join(testFileLocation, preParamTestFile))
	if err != nil {
		logger.Error().Err(err).Msgf("fail to open file")
	}
	preParamsStr := strings.Split(string(buf), "\n")
	for _, item := range preParamsStr {
		var preParam btsskeygen.LocalPreParams
		val, err := hex.DecodeString(item)
		if err != nil {
			logger.Error().Err(err).Msgf("fail to decode item")
		}
		if json.Unmarshal(val, &preParam) != nil {
			logger.Error().Err(err).Msgf("fail to decode pre-parameter")
		}
		preParamArray = append(preParamArray, &preParam)
	}
	return preParamArray
}

// generate a new key
func (s *FourDemoSuite) moneroDoTestKeygen(newJoinParty bool) error {
	wg := sync.WaitGroup{}
	lock := &sync.Mutex{}
	var globalErr error
	keygenResult := make(map[int]moneroKeygen.Response)
	for i := 0; i < partyNum; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var req moneroKeygen.Request
			if newJoinParty {
				req = moneroKeygen.NewRequest(testPubKeys, 10, "0.14.0", s.rpcAddress[idx])
			} else {
				req = moneroKeygen.NewRequest(testPubKeys, 10, "0.13.0", s.rpcAddress[idx])
			}
			res, err := s.moneroServers[idx].Keygen(req)
			if err != nil {
				s.logger.Error().Err(err).Msgf("fail to do monero keygen")
				globalErr = err
			}
			lock.Lock()
			defer lock.Unlock()
			keygenResult[idx] = res
		}(i)
	}
	wg.Wait()
	fmt.Printf(">>>>>>>>>>pool address %v\n", keygenResult[0].PoolAddress)
	return globalErr
}

func main() {
	instance := FourDemoSuite{}

	defer func() {
		instance.TearDownTest()
	}()

	err := instance.SetUpTest()
	if err != nil {
		fmt.Printf("system fail to start")
	}

	instance.doTestKeygen(true)

}
