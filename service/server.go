package service

import (
	jsoniter "github.com/json-iterator/go"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/airportr/miaospeed/interfaces"
	"github.com/airportr/miaospeed/preconfigs"
	"github.com/airportr/miaospeed/utils"
	"github.com/airportr/miaospeed/utils/ipfliter"
	"github.com/airportr/miaospeed/utils/structs"
	"github.com/gorilla/websocket"

	"github.com/airportr/miaospeed/service/matrices"
	"github.com/airportr/miaospeed/service/taskpoll"
)

type WsHandler struct {
	Serve    func(http.ResponseWriter, *http.Request)
	IPFilter *ipfliter.IPFilter
}

func (wh *WsHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	//show simple forbidden text
	if !wh.IPFilter.Allowed(ip) {
		http.Error(rw, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if wh.Serve != nil {
		wh.Serve(rw, r)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func InitServer() {
	if utils.GCFG.Binder == "" {
		utils.DErrorf("MiaoSpeed Server | Cannot listening the binder, bind=%s", utils.GCFG.Binder)
		os.Exit(1)
	}

	utils.DWarnf("MiaoSpeed Server | Start Listening, bind=%s", utils.GCFG.Binder)
	wsHandler := WsHandler{
		IPFilter: ipfliter.New(ipfliter.Options{
			AllowedIPs:     utils.GCFG.AllowIPs,
			BlockByDefault: true,
		}),
		Serve: func(rw http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(rw, r, nil)
			if err != nil {
				utils.DErrorf("MiaoServer Test | Socket establishing error, error=%s", err.Error())
				return
			}
			defer func() {
				_ = conn.Close()
			}()
			utils.DWarnf("MiaoServer | new unverified connection | remote=%s", r.RemoteAddr)

			// Verify the websocket path
			if !utils.GCFG.ValidateWSPath(r.URL.Path) {
				_ = conn.WriteJSON(&interfaces.SlaveResponse{
					Error: "invalid websocket path",
				})
				utils.DWarnf("MiaoServer Test | websocket path error, error=%s", "invalid websocket path")
				return
			}

			var poll *taskpoll.TPController
			batches := structs.NewAsyncMap[string, bool]()

			cancel := func() {
				if poll != nil {
					for id := range batches.ForEach() {
						poll.Remove(id, taskpoll.TPExitInterrupt)
					}
				}
			}
			defer cancel()

			for {
				sr := interfaces.SlaveRequest{}
				_, r2, err := conn.NextReader()
				if err == nil {
					// unsafe, ensure jsoniter package receives frequently security updates.
					err = jsoniter.NewDecoder(r2).Decode(&sr)
					if err == io.EOF {
						// One value is expected in the message.
						err = io.ErrUnexpectedEOF
					}
				}
				if err != nil {
					if !strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(), "reset by peer") {
						utils.DErrorf("MiaoServer Test | Task receiving error, error=%s", err.Error())
					}
					return
				}

				// let me see see miaoko's test script and nodes! 
				verified := utils.GCFG.VerifyRequest(&sr)

				utils.DWarnf("MiaoServer Test | Nodes: %v", sr.Nodes)

				mtrx := matrices.FindBatchFromEntry(sr.Options.Matrices)
				macros := ExtractMacrosFromMatrices(mtrx)
				utils.DWarnf("MiaoServer Test | Matrices: %v, Macros: %v", sr.Options.Matrices, macros)

				scripts := sr.Configs.Check().Scripts
				if len(scripts) > 0 {
			    	scriptsBytes, _ := jsoniter.MarshalIndent(scripts, "", "  ")
				    utils.DWarnf("MiaoServer Test | Scripts:\n-----BEGIN SCRIPTS-----\n%s\n-----END SCRIPTS-----", string(scriptsBytes))
				}

				utils.DWarnf("MiaoServer Test | Receive Task, name=%s invoker=%v verify=%v remote=%s payload_nodes=%d payload_scripts=%d",
					sr.Basics.ID, sr.Basics.Invoker, verified, r.RemoteAddr, len(sr.Nodes), len(scripts))

				// verify token
				if !verified {
					_ = conn.WriteJSON(&interfaces.SlaveResponse{
						Error: "cannot verify the request, please check your token",
					})
					return
				}
				sr.Challenge = ""

				// verify invoker
				if !utils.GCFG.InWhiteList(sr.Basics.Invoker) {
					_ = conn.WriteJSON(&interfaces.SlaveResponse{
						Error: "the bot id is not in the whitelist",
					})
					return
				}

				// select poll
				if structs.Contains(macros, interfaces.MacroSpeed) || structs.Contains(macros, interfaces.MacroSleep) {
					if utils.GCFG.NoSpeedFlag {
						_ = conn.WriteJSON(&interfaces.SlaveResponse{
							Error: "speedtest is disabled on backend",
						})
						return
					}
					poll = SpeedTaskPoll
				} else {
					poll = ConnTaskPoll
				}

				utils.DWarnf("MiaoServer Test | Receive Task, name=%s poll=%s", sr.Basics.ID, poll.Name())

				// build testing item
				item := poll.Push((&TestingPollItem{
					id:       utils.RandomUUID(),
					name:     sr.Basics.ID,
					request:  &sr,
					matrices: sr.Options.Matrices,
					macros:   macros,
					onProcess: func(self *TestingPollItem, idx int, result interfaces.SlaveEntrySlot) {
						_ = conn.WriteJSON(&interfaces.SlaveResponse{
							ID:               self.ID(),
							MiaoSpeedVersion: utils.VERSION,
							Progress: &interfaces.SlaveProgress{
								Record:  result,
								Index:   idx,
								Queuing: poll.AwaitingCount(),
							},
						})
					},
					onExit: func(self *TestingPollItem, exitCode taskpoll.TPExitCode) {
						batches.Del(self.ID())
						_ = conn.WriteJSON(&interfaces.SlaveResponse{
							ID:               self.ID(),
							MiaoSpeedVersion: utils.VERSION,
							Result: &interfaces.SlaveTask{
								Request: sr,
								Results: self.results.ForEach(),
							},
						})
					},
					calcWeight: func(self *TestingPollItem) uint {
						return 1
					},
				}).Init())

				// onstart
				if sr.Configs.ApiVersion == 2 {
					_ = conn.WriteJSON(&interfaces.SlaveResponse{
						ID:               item.ID(),
						MiaoSpeedVersion: utils.VERSION,
						Progress: &interfaces.SlaveProgress{
							Queuing: poll.UnsafeAwaitingCount(),
						},
					})
				}
				batches.Set(item.ID(), true)
			}
		},
	}

	server := http.Server{
		Handler:   &wsHandler,
		TLSConfig: preconfigs.MakeSelfSignedTLSServer(),
	}

	if strings.HasPrefix(utils.GCFG.Binder, "/") {
		unixListener, err := net.Listen("unix", utils.GCFG.Binder)
		if err != nil {
			utils.DErrorf("MiaoServer Launch | Cannot listen on unixsocket %s, error=%s", utils.GCFG.Binder, err.Error())
			os.Exit(1)
		}
		err = server.Serve(unixListener)
		if err != nil {
			utils.DErrorf("MiaoServer Launch | Cannot serve on unixsocket %s, error=%s", utils.GCFG.Binder, err.Error())
		}
	} else {
		netListener, err := net.Listen("tcp", utils.GCFG.Binder)
		if err != nil {
			utils.DErrorf("MiaoServer Launch | Cannot listen on socket %s, error=%s", utils.GCFG.Binder, err.Error())
			os.Exit(1)
		}
		if utils.GCFG.MiaoKoSignedTLS {
			err := server.ServeTLS(netListener, "", "")
			if err != nil {
				utils.DErrorf("MiaoServer Launch | Cannot serve on socket %s, error=%s", utils.GCFG.Binder, err.Error())
			}
		} else {
			err := server.Serve(netListener)
			if err != nil {
				utils.DErrorf("MiaoServer Launch | Cannot serve on socket %s, error=%s", utils.GCFG.Binder, err.Error())
			}
		}

	}
}

func CleanUpServer() {
	if strings.HasPrefix(utils.GCFG.Binder, "/") {
		err := os.Remove(utils.GCFG.Binder)
		if err != nil {
			utils.DErrorf("MiaoServer CleanUp OS Error | Cannot remove unixsocket %s, error=%s", utils.GCFG.Binder, err.Error())
		}
	}
}