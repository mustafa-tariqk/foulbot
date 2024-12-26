INSERT INTO
    polls (
        channel_id,
        message_id,
        creator_id,
        points,
        reason,
        expiry,
        passed
    )
VALUES
    (?, ?, ?, ?, ?, ?, NULL);
