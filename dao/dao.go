package dao

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed make_tables.sql
var makeTablesQuery string

//go:embed insert_poll.sql
var insertPollQuery string

//go:embed insert_gainers.sql
var insertGainersQuery string

//go:embed vote.sql
var voteQuery string

//go:embed leaderboard.sql
var leaderboardQuery string

//go:embed expired_rows.sql
var expiredRowsQuery string

//go:embed collect_votes.sql
var collectVotesQuery string

//go:embed finalize_poll.sql
var finalizePollQuery string

var db *sql.DB
var err error

type Poll struct {
	MesssageId string
	ChannelId  string
	CreatorId  string
	Points     int64
	Reason     string
	GainderIds []string
	Expiry     string
}

type EvaluatedPoll struct {
	MesssageId   string
	ChannelId    string
	CreatorId    string
	Points       int64
	Reason       string
	GainderIds   []string
	VotesFor     []string
	VotesAgainst []string
	Passed       bool
	Expiry       string
}

type Position struct {
	UserId string
	Points int64
}

func init() {
	var err error
	// https://briandouglas.ie/sqlite-defaults/
	db, err = sql.Open("sqlite", `file:foulbot.sqlite?
            _journal_mode=WAL&
            _synchronous=NORMAL&
            _busy_timeout=5000&
            _cache_size=-20000&
            _foreign_keys=ON&
            _auto_vacuum=INCREMENTAL&
            _temp_store=MEMORY&
            _mmap_size=2147483648&
            _page_size=8192`)
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(makeTablesQuery)
	if err != nil {
		panic(err)
	}
}

func CreatePoll(poll Poll) {
	_, err = db.Exec(insertPollQuery, poll.ChannelId, poll.MesssageId, poll.CreatorId, poll.Points, poll.Reason, poll.Expiry)
	if err != nil {
		panic(err)
	}

	for _, gainerId := range poll.GainderIds {
		_, err = db.Exec(insertGainersQuery, poll.ChannelId, poll.MesssageId, gainerId)
		if err != nil {
			panic(err)
		}
	}
}

func Vote(channelId, messageId, voterId string, vote bool) {
	_, err = db.Exec(voteQuery, channelId, messageId, voterId, vote)
	if err != nil {
		panic(err)
	}
}

func EvaluatePolls() (polls []EvaluatedPoll) {
	expiredRows, err := db.Query(expiredRowsQuery)
	if err != nil {
		panic(err)
	}
	defer expiredRows.Close()
	for expiredRows.Next() {
		var poll EvaluatedPoll
		err = expiredRows.Scan(&poll.MesssageId, &poll.ChannelId, &poll.CreatorId, &poll.Points, &poll.Reason, &poll.Expiry)
		if err != nil {
			panic(err)
		}
		polls = append(polls, poll)
	}

	for i := range polls {
		poll := &polls[i]
		votesForRows, err := db.Query(collectVotesQuery, poll.ChannelId, poll.MesssageId, 1)
		if err != nil {
			panic(err)
		}
		defer votesForRows.Close()
		for votesForRows.Next() {
			var voterId string
			err = votesForRows.Scan(&voterId)
			if err != nil {
				panic(err)
			}
			poll.VotesFor = append(poll.VotesFor, voterId)
		}

		votesAgainstRows, err := db.Query(collectVotesQuery, poll.ChannelId, poll.MesssageId, 0)
		if err != nil {
			panic(err)
		}
		defer votesAgainstRows.Close()
		for votesAgainstRows.Next() {
			var voterId string
			err = votesAgainstRows.Scan(&voterId)
			if err != nil {
				panic(err)
			}
			poll.VotesAgainst = append(poll.VotesAgainst, voterId)
		}

		if len(poll.VotesFor) > len(poll.VotesAgainst) {
			poll.Passed = true
		} else {
			poll.Passed = false
		}
		_, err = db.Exec(finalizePollQuery, poll.Passed, poll.ChannelId, poll.MesssageId)
		if err != nil {
			panic(err)
		}
	}
	return polls
}

func Leaderboard(year string) (podium []Position) {
	rows, err := db.Query(leaderboardQuery, year)

	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var position Position
		err = rows.Scan(&position.UserId, &position.Points)
		if err != nil {
			panic(err)
		}
		podium = append(podium, position)
	}
	return podium
}
