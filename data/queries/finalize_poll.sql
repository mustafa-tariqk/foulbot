UPDATE polls
SET
    passed = ?
WHERE
    channel_id = ?
    AND message_id = ?;
