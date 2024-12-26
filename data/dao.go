package data

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed queries/make_tables.sql
var makeTablesQuery string

//go:embed queries/insert_poll.sql
var insertPollQuery string

//go:embed queries/insert_gainers.sql
var insertGainersQuery string

//go:embed queries/vote.sql
var voteQuery string

//go:embed queries/leaderboard.sql
var leaderboardQuery string

//go:embed queries/expired_rows.sql
var expiredRowsQuery string

//go:embed queries/collect_votes.sql
var collectVotesQuery string

//go:embed queries/finalize_poll.sql
var finalizePollQuery string

//go:embed queries/collect_gainers.sql
var collectGainersQuery string

var db *sql.DB
var err error

type Poll struct {
	MessageId string
	ChannelId string
	CreatorId string
	Points    int64
	Reason    string
	GainerIds []string
	Expiry    string
}

type EvaluatedPoll struct {
	MessageId    string
	ChannelId    string
	CreatorId    string
	Points       int64
	Reason       string
	GainerIds    []string
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
	_, err = db.Exec(insertPollQuery, poll.ChannelId, poll.MessageId, poll.CreatorId, poll.Points, poll.Reason, poll.Expiry)
	if err != nil {
		panic(err)
	}

	for _, gainerId := range poll.GainerIds {
		_, err = db.Exec(insertGainersQuery, poll.ChannelId, poll.MessageId, gainerId)
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
		err = expiredRows.Scan(&poll.MessageId, &poll.ChannelId, &poll.CreatorId, &poll.Points, &poll.Reason, &poll.Expiry)
		if err != nil {
			panic(err)
		}
		polls = append(polls, poll)
	}

	for i := range polls {
		poll := &polls[i]
		votesForRows, err := db.Query(collectVotesQuery, poll.ChannelId, poll.MessageId, 1)
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

		votesAgainstRows, err := db.Query(collectVotesQuery, poll.ChannelId, poll.MessageId, 0)
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

		rows, err := db.Query(collectGainersQuery, poll.ChannelId, poll.MessageId)
		if err != nil {
			panic(err)
		}
		defer rows.Close()
		for rows.Next() {
			var gainerId string
			err = rows.Scan(&gainerId)
			if err != nil {
				panic(err)
			}
			poll.GainerIds = append(poll.GainerIds, gainerId)
		}

		_, err = db.Exec(finalizePollQuery, poll.Passed, poll.ChannelId, poll.MessageId)
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
