// Copyright 2024 Daytona Platforms Inc.
// SPDX-License-Identifier: Apache-2.0

package apiclient

import (
	"fmt"
	"sync"
	"time"

	"github.com/daytonaio/daytona/cmd/daytona/config"
	"github.com/daytonaio/daytona/pkg/logs"
	logs_view "github.com/daytonaio/daytona/pkg/views/logs"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

var workspaceLogsStarted bool

func ReadWorkspaceLogs(activeProfile config.Profile, workspaceId string, projectNames []string, stopLogs *bool) {
	var wg sync.WaitGroup
	query := "follow=true&retry=true"

	logs_view.CalculateLongestPrefixLength(projectNames)

	for index, projectName := range projectNames {
		wg.Add(1)
		go func(projectName string) {
			defer wg.Done()

			for {
				// Make sure workspace logs started before showing any project logs
				if !workspaceLogsStarted {
					time.Sleep(250 * time.Millisecond)
					continue
				}

				ws, res, err := GetWebsocketConn(fmt.Sprintf("/log/workspace/%s/%s", workspaceId, projectName), &activeProfile, &query)
				// We want to retry getting the logs if it fails
				if err != nil {
					log.Trace(HandleErrorResponse(res, err))
					time.Sleep(500 * time.Millisecond)
					continue
				}

				ReadJSONLog(ws, index, stopLogs)
				ws.Close()
				break
			}
		}(projectName)
	}

	for {
		ws, res, err := GetWebsocketConn(fmt.Sprintf("/log/workspace/%s", workspaceId), &activeProfile, &query)
		// We want to retry getting the logs if it fails
		if err != nil {
			log.Trace(HandleErrorResponse(res, err))
			time.Sleep(250 * time.Millisecond)
			continue
		}

		ReadJSONLog(ws, logs_view.WORKSPACE_INDEX, stopLogs)
		ws.Close()
		break
	}

	wg.Wait()
}

func ReadJSONLog(ws *websocket.Conn, index int, stopLogs ...*bool) {
	logEntriesChan := make(chan logs.LogEntry)
	go logs_view.DisplayLogs(logEntriesChan, index)
	for {
		var logEntry logs.LogEntry
		err := ws.ReadJSON(&logEntry)
		if err != nil {
			return
		}
		logEntriesChan <- logEntry
		if !workspaceLogsStarted && index == logs_view.WORKSPACE_INDEX {
			workspaceLogsStarted = true
		}
		if len(stopLogs) > 0 && stopLogs[0] != nil && *stopLogs[0] {
			return
		}
	}
}
