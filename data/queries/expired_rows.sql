SELECT
    *
FROM
    polls
WHERE
    passed is NULL
    AND EXPIRY < datetime ('now');
