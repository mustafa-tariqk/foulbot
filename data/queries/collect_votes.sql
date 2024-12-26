SELECT
    user_id
FROM
    votes
WHERE
    channel_id = ?
    AND message_id = ?
    AND value = ?;
