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
    AND EXPIRY < datetime ('now');
