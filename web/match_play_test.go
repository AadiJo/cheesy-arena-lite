// Copyright 2014 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)

package web

import (
	"bytes"
	"fmt"
	"github.com/Team254/cheesy-arena/field"
	"github.com/Team254/cheesy-arena/game"
	"github.com/Team254/cheesy-arena/model"
	"github.com/Team254/cheesy-arena/tournament"
	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"log"
	"sync"
	"testing"
	"time"
)

func TestMatchPlay(t *testing.T) {
	web := setupTestWeb(t)

	match1 := model.Match{Type: "practice", DisplayName: "1", Status: "complete", Winner: "R"}
	match2 := model.Match{Type: "practice", DisplayName: "2"}
	match3 := model.Match{Type: "qualification", DisplayName: "1", Status: "complete", Winner: "B"}
	match4 := model.Match{Type: "elimination", DisplayName: "SF1-1", Status: "complete", Winner: "T"}
	match5 := model.Match{Type: "elimination", DisplayName: "SF1-2"}
	web.arena.Database.CreateMatch(&match1)
	web.arena.Database.CreateMatch(&match2)
	web.arena.Database.CreateMatch(&match3)
	web.arena.Database.CreateMatch(&match4)
	web.arena.Database.CreateMatch(&match5)

	// Check that all matches are listed on the page.
	recorder := web.getHttpResponse("/match_play")
	assert.Equal(t, 200, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "P1")
	assert.Contains(t, recorder.Body.String(), "P2")
	assert.Contains(t, recorder.Body.String(), "Q1")
	assert.Contains(t, recorder.Body.String(), "SF1-1")
	assert.Contains(t, recorder.Body.String(), "SF1-2")
}

func TestMatchPlayLoad(t *testing.T) {
	web := setupTestWeb(t)

	web.arena.Database.CreateTeam(&model.Team{Id: 101})
	web.arena.Database.CreateTeam(&model.Team{Id: 102})
	web.arena.Database.CreateTeam(&model.Team{Id: 103})
	web.arena.Database.CreateTeam(&model.Team{Id: 104})
	web.arena.Database.CreateTeam(&model.Team{Id: 105})
	web.arena.Database.CreateTeam(&model.Team{Id: 106})
	match := model.Match{Type: "elimination", DisplayName: "QF4-3", Status: "complete", Winner: "R", Red1: 101,
		Red2: 102, Red3: 103, Blue1: 104, Blue2: 105, Blue3: 106}
	web.arena.Database.CreateMatch(&match)
	recorder := web.getHttpResponse("/match_play")
	assert.Equal(t, 200, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "101")
	assert.NotContains(t, recorder.Body.String(), "102")
	assert.NotContains(t, recorder.Body.String(), "103")
	assert.NotContains(t, recorder.Body.String(), "104")
	assert.NotContains(t, recorder.Body.String(), "105")
	assert.NotContains(t, recorder.Body.String(), "106")

	// Load the match and check for the team numbers again.
	recorder = web.getHttpResponse(fmt.Sprintf("/match_play/%d/load", match.Id))
	assert.Equal(t, 303, recorder.Code)
	recorder = web.getHttpResponse("/match_play")
	assert.Equal(t, 200, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "101")
	assert.Contains(t, recorder.Body.String(), "102")
	assert.Contains(t, recorder.Body.String(), "103")
	assert.Contains(t, recorder.Body.String(), "104")
	assert.Contains(t, recorder.Body.String(), "105")
	assert.Contains(t, recorder.Body.String(), "106")

	// Load a test match.
	recorder = web.getHttpResponse("/match_play/0/load")
	assert.Equal(t, 303, recorder.Code)
	recorder = web.getHttpResponse("/match_play")
	assert.Equal(t, 200, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "101")
	assert.NotContains(t, recorder.Body.String(), "102")
	assert.NotContains(t, recorder.Body.String(), "103")
	assert.NotContains(t, recorder.Body.String(), "104")
	assert.NotContains(t, recorder.Body.String(), "105")
	assert.NotContains(t, recorder.Body.String(), "106")
}

func TestMatchPlayShowResult(t *testing.T) {
	web := setupTestWeb(t)

	recorder := web.getHttpResponse("/match_play/1/show_result")
	assert.Equal(t, 500, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Invalid match")
	match := model.Match{Type: "qualification", DisplayName: "1", Status: "complete"}
	web.arena.Database.CreateMatch(&match)
	recorder = web.getHttpResponse(fmt.Sprintf("/match_play/%d/show_result", match.Id))
	assert.Equal(t, 500, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "No result found")
	web.arena.Database.CreateMatchResult(&model.MatchResult{MatchId: match.Id})
	recorder = web.getHttpResponse(fmt.Sprintf("/match_play/%d/show_result", match.Id))
	assert.Equal(t, 303, recorder.Code)
	assert.Equal(t, match.Id, web.arena.SavedMatch.Id)
	assert.Equal(t, match.Id, web.arena.SavedMatchResult.MatchId)
}

func TestMatchPlayErrors(t *testing.T) {
	web := setupTestWeb(t)

	// Load an invalid match.
	recorder := web.getHttpResponse("/match_play/1114/load")
	assert.Equal(t, 500, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Invalid match")
}

func TestCommitMatch(t *testing.T) {
	web := setupTestWeb(t)

	// Committing test match should do nothing.
	match := &model.Match{Id: 0, Type: "test", Red1: 101, Red2: 102, Red3: 103, Blue1: 104, Blue2: 105, Blue3: 106}
	err := web.commitMatchScore(match, &model.MatchResult{MatchId: match.Id}, false)
	assert.Nil(t, err)
	matchResult, err := web.arena.Database.GetMatchResultForMatch(match.Id)
	assert.Nil(t, err)
	assert.Nil(t, matchResult)

	// Committing the same match more than once should create a second match result record.
	match.Id = 1
	match.Type = "qualification"
	web.arena.Database.CreateMatch(match)
	matchResult = model.NewMatchResult()
	matchResult.MatchId = match.Id
	matchResult.BlueScore = &game.Score{AutoRuns: 2}
	err = web.commitMatchScore(match, matchResult, false)
	assert.Nil(t, err)
	assert.Equal(t, 1, matchResult.PlayNumber)
	match, _ = web.arena.Database.GetMatchById(1)
	assert.Equal(t, "B", match.Winner)

	matchResult = model.NewMatchResult()
	matchResult.MatchId = match.Id
	matchResult.RedScore = &game.Score{AutoRuns: 1}
	err = web.commitMatchScore(match, matchResult, false)
	assert.Nil(t, err)
	assert.Equal(t, 2, matchResult.PlayNumber)
	match, _ = web.arena.Database.GetMatchById(1)
	assert.Equal(t, "R", match.Winner)

	matchResult = model.NewMatchResult()
	matchResult.MatchId = match.Id
	err = web.commitMatchScore(match, matchResult, false)
	assert.Nil(t, err)
	assert.Equal(t, 3, matchResult.PlayNumber)
	match, _ = web.arena.Database.GetMatchById(1)
	assert.Equal(t, "T", match.Winner)

	// Verify TBA and STEMtv publishing by checking the log for the expected failure messages.
	web.arena.TbaClient.BaseUrl = "fakeUrl"
	web.arena.StemTvClient.BaseUrl = "fakeUrl"
	web.arena.EventSettings.TbaPublishingEnabled = true
	web.arena.EventSettings.StemTvPublishingEnabled = true
	var writer bytes.Buffer
	log.SetOutput(&writer)
	err = web.commitMatchScore(match, matchResult, false)
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 10) // Allow some time for the asynchronous publishing to happen.
	assert.Contains(t, writer.String(), "Failed to publish matches")
	assert.Contains(t, writer.String(), "Failed to publish rankings")
	assert.Contains(t, writer.String(), "Failed to publish match video split to STEMtv")
}

func TestCommitEliminationTie(t *testing.T) {
	web := setupTestWeb(t)

	match := &model.Match{Id: 0, Type: "qualification", Red1: 1, Red2: 2, Red3: 3, Blue1: 4, Blue2: 5, Blue3: 6}
	web.arena.Database.CreateMatch(match)
	matchResult := &model.MatchResult{MatchId: match.Id, RedScore: &game.Score{ForceCubes: 1, Fouls: []game.Foul{{}}},
		BlueScore: &game.Score{}}
	err := web.commitMatchScore(match, matchResult, false)
	assert.Nil(t, err)
	match, _ = web.arena.Database.GetMatchById(1)
	assert.Equal(t, "T", match.Winner)
	match.Type = "elimination"
	web.arena.Database.SaveMatch(match)
	web.commitMatchScore(match, matchResult, false)
	match, _ = web.arena.Database.GetMatchById(1)
	assert.Equal(t, "T", match.Winner) // No elimination tiebreakers.
}

func TestCommitCards(t *testing.T) {
	web := setupTestWeb(t)

	// Check that a yellow card sticks with a team.
	team := &model.Team{Id: 5}
	web.arena.Database.CreateTeam(team)
	match := &model.Match{Id: 0, Type: "qualification", Red1: 1, Red2: 2, Red3: 3, Blue1: 4, Blue2: 5, Blue3: 6}
	web.arena.Database.CreateMatch(match)
	matchResult := model.NewMatchResult()
	matchResult.MatchId = match.Id
	matchResult.BlueCards = map[string]string{"5": "yellow"}
	err := web.commitMatchScore(match, matchResult, false)
	assert.Nil(t, err)
	team, _ = web.arena.Database.GetTeamById(5)
	assert.True(t, team.YellowCard)

	// Check that editing a match result removes a yellow card from a team.
	matchResult = model.NewMatchResult()
	matchResult.MatchId = match.Id
	err = web.commitMatchScore(match, matchResult, false)
	assert.Nil(t, err)
	team, _ = web.arena.Database.GetTeamById(5)
	assert.False(t, team.YellowCard)

	// Check that a red card causes a yellow card to stick with a team.
	matchResult = model.NewMatchResult()
	matchResult.MatchId = match.Id
	matchResult.BlueCards = map[string]string{"5": "red"}
	err = web.commitMatchScore(match, matchResult, false)
	assert.Nil(t, err)
	team, _ = web.arena.Database.GetTeamById(5)
	assert.True(t, team.YellowCard)

	// Check that a red card in eliminations zeroes out the score.
	tournament.CreateTestAlliances(web.arena.Database, 2)
	match.Type = "elimination"
	web.arena.Database.SaveMatch(match)
	matchResult = model.BuildTestMatchResult(match.Id, 10)
	matchResult.MatchType = match.Type
	matchResult.RedCards = map[string]string{"1": "red"}
	err = web.commitMatchScore(match, matchResult, false)
	assert.Nil(t, err)
	assert.Equal(t, 0, matchResult.RedScoreSummary().Score)
	assert.NotEqual(t, 0, matchResult.BlueScoreSummary().Score)
}

func TestMatchPlayWebsocketCommands(t *testing.T) {
	web := setupTestWeb(t)

	server, wsUrl := web.startTestServer()
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial(wsUrl+"/match_play/websocket", nil)
	assert.Nil(t, err)
	defer conn.Close()
	ws := &Websocket{conn, new(sync.Mutex)}

	// Should get a few status updates right after connection.
	readWebsocketType(t, ws, "status")
	readWebsocketType(t, ws, "matchTiming")
	readWebsocketType(t, ws, "matchTime")
	readWebsocketType(t, ws, "realtimeScore")
	readWebsocketType(t, ws, "setAudienceDisplay")
	readWebsocketType(t, ws, "scoringStatus")
	readWebsocketType(t, ws, "setAllianceStationDisplay")

	// Test that a server-side error is communicated to the client.
	ws.Write("nonexistenttype", nil)
	assert.Contains(t, readWebsocketError(t, ws), "Invalid message type")

	// Test match setup commands.
	ws.Write("substituteTeam", nil)
	assert.Contains(t, readWebsocketError(t, ws), "Invalid alliance station")
	ws.Write("substituteTeam", map[string]interface{}{"team": 254, "position": "B5"})
	assert.Contains(t, readWebsocketError(t, ws), "Invalid alliance station")
	ws.Write("substituteTeam", map[string]interface{}{"team": 254, "position": "B1"})
	readWebsocketType(t, ws, "status")
	assert.Equal(t, 254, web.arena.CurrentMatch.Blue1)
	ws.Write("substituteTeam", map[string]interface{}{"team": 0, "position": "B1"})
	readWebsocketType(t, ws, "status")
	assert.Equal(t, 0, web.arena.CurrentMatch.Blue1)
	ws.Write("toggleBypass", nil)
	assert.Contains(t, readWebsocketError(t, ws), "Failed to parse")
	ws.Write("toggleBypass", "R4")
	assert.Contains(t, readWebsocketError(t, ws), "Invalid alliance station")
	ws.Write("toggleBypass", "R3")
	readWebsocketType(t, ws, "status")
	assert.Equal(t, true, web.arena.AllianceStations["R3"].Bypass)
	ws.Write("toggleBypass", "R3")
	readWebsocketType(t, ws, "status")
	assert.Equal(t, false, web.arena.AllianceStations["R3"].Bypass)

	// Go through match flow.
	ws.Write("abortMatch", nil)
	assert.Contains(t, readWebsocketError(t, ws), "Cannot abort match")
	ws.Write("startMatch", nil)
	assert.Contains(t, readWebsocketError(t, ws), "Cannot start match")
	web.arena.AllianceStations["R1"].Bypass = true
	web.arena.AllianceStations["R2"].Bypass = true
	web.arena.AllianceStations["R3"].Bypass = true
	web.arena.AllianceStations["B1"].Bypass = true
	web.arena.AllianceStations["B2"].Bypass = true
	web.arena.AllianceStations["B3"].Bypass = true
	ws.Write("startMatch", nil)
	readWebsocketType(t, ws, "status")
	assert.Equal(t, field.StartMatch, web.arena.MatchState)
	ws.Write("commitResults", nil)
	assert.Contains(t, readWebsocketError(t, ws), "Cannot reset match")
	ws.Write("discardResults", nil)
	assert.Contains(t, readWebsocketError(t, ws), "Cannot reset match")
	ws.Write("abortMatch", nil)
	readWebsocketType(t, ws, "status")
	readWebsocketType(t, ws, "setAudienceDisplay")
	assert.Equal(t, field.PostMatch, web.arena.MatchState)
	web.arena.RedRealtimeScore.CurrentScore.AutoRuns = 1
	web.arena.BlueRealtimeScore.CurrentScore.BoostCubes = 2
	ws.Write("commitResults", nil)
	readWebsocketMultiple(t, ws, 3) // reload, realtimeScore, setAllianceStationDisplay
	assert.Equal(t, 1, web.arena.SavedMatchResult.RedScore.AutoRuns)
	assert.Equal(t, 2, web.arena.SavedMatchResult.BlueScore.BoostCubes)
	assert.Equal(t, field.PreMatch, web.arena.MatchState)
	ws.Write("discardResults", nil)
	readWebsocketMultiple(t, ws, 3) // reload, realtimeScore, setAllianceStationDisplay
	assert.Equal(t, field.PreMatch, web.arena.MatchState)

	// Test changing the displays.
	ws.Write("setAudienceDisplay", "logo")
	readWebsocketType(t, ws, "setAudienceDisplay")
	assert.Equal(t, "logo", web.arena.AudienceDisplayScreen)
	ws.Write("setAllianceStationDisplay", "logo")
	readWebsocketType(t, ws, "setAllianceStationDisplay")
	assert.Equal(t, "logo", web.arena.AllianceStationDisplayScreen)
}

func TestMatchPlayWebsocketNotifications(t *testing.T) {
	web := setupTestWeb(t)

	web.arena.Database.CreateTeam(&model.Team{Id: 254})

	server, wsUrl := web.startTestServer()
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial(wsUrl+"/match_play/websocket", nil)
	assert.Nil(t, err)
	defer conn.Close()
	ws := &Websocket{conn, new(sync.Mutex)}

	// Should get a few status updates right after connection.
	readWebsocketType(t, ws, "status")
	readWebsocketType(t, ws, "matchTiming")
	readWebsocketType(t, ws, "matchTime")
	readWebsocketType(t, ws, "realtimeScore")
	readWebsocketType(t, ws, "setAudienceDisplay")
	readWebsocketType(t, ws, "scoringStatus")

	web.arena.AllianceStations["R1"].Bypass = true
	web.arena.AllianceStations["R2"].Bypass = true
	web.arena.AllianceStations["R3"].Bypass = true
	web.arena.AllianceStations["B1"].Bypass = true
	web.arena.AllianceStations["B2"].Bypass = true
	web.arena.AllianceStations["B3"].Bypass = true
	web.arena.StartMatch()
	web.arena.Update()
	messages := readWebsocketMultiple(t, ws, 3)
	_, ok := messages["matchTime"]
	assert.True(t, ok)
	_, ok = messages["setAudienceDisplay"]
	assert.True(t, ok)
	_, ok = messages["setAllianceStationDisplay"]
	web.arena.MatchStartTime = time.Now().Add(-time.Duration(game.MatchTiming.WarmupDurationSec) * time.Second)
	web.arena.Update()
	messages = readWebsocketMultiple(t, ws, 2)
	statusReceived, matchTime := getStatusMatchTime(t, messages)
	assert.Equal(t, true, statusReceived)
	assert.Equal(t, 3, matchTime.MatchState)
	assert.Equal(t, 3, matchTime.MatchTimeSec)
	assert.True(t, ok)
	web.arena.ScoringStatusNotifier.Notify(nil)
	readWebsocketType(t, ws, "scoringStatus")

	// Should get a tick notification when an integer second threshold is crossed.
	web.arena.MatchStartTime = time.Now().Add(-time.Second - 10*time.Millisecond) // Crossed
	web.arena.Update()
	err = mapstructure.Decode(readWebsocketType(t, ws, "matchTime"), &matchTime)
	assert.Nil(t, err)
	assert.Equal(t, 3, matchTime.MatchState)
	assert.Equal(t, 1, matchTime.MatchTimeSec)
	web.arena.MatchStartTime = time.Now().Add(-2*time.Second + 10*time.Millisecond) // Not crossed yet
	web.arena.Update()
	web.arena.MatchStartTime = time.Now().Add(-2*time.Second - 10*time.Millisecond) // Crossed
	web.arena.Update()
	err = mapstructure.Decode(readWebsocketType(t, ws, "matchTime"), &matchTime)
	assert.Nil(t, err)
	assert.Equal(t, 3, matchTime.MatchState)
	assert.Equal(t, 2, matchTime.MatchTimeSec)

	// Check across a match state boundary.
	web.arena.MatchStartTime = time.Now().Add(-time.Duration(game.MatchTiming.WarmupDurationSec+
		game.MatchTiming.AutoDurationSec) * time.Second)
	web.arena.Update()
	statusReceived, matchTime = readWebsocketStatusMatchTime(t, ws)
	assert.Equal(t, true, statusReceived)
	assert.Equal(t, 4, matchTime.MatchState)
	assert.Equal(t, game.MatchTiming.WarmupDurationSec+game.MatchTiming.AutoDurationSec, matchTime.MatchTimeSec)
}

// Handles the status and matchTime messages arriving in either order.
func readWebsocketStatusMatchTime(t *testing.T, ws *Websocket) (bool, MatchTimeMessage) {
	return getStatusMatchTime(t, readWebsocketMultiple(t, ws, 2))
}

func getStatusMatchTime(t *testing.T, messages map[string]interface{}) (bool, MatchTimeMessage) {
	_, statusReceived := messages["status"]
	message, ok := messages["matchTime"]
	var matchTime MatchTimeMessage
	if assert.True(t, ok) {
		err := mapstructure.Decode(message, &matchTime)
		assert.Nil(t, err)
	}
	return statusReceived, matchTime
}
