SELECT
    message_id,
    channel_id,
    creator_id,
    points,
    reason,
    expiry
FROM
    polls
WHERE
    passed is NULL
    AND EXPIRY < strftime ('%Y-%m-%dT%H:%M:%S-0500', 'now', 'localtime');