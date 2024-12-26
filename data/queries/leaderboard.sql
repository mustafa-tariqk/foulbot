SELECT
    g.user_id,
    SUM(p.points) as total_points
FROM
    polls p
    JOIN gainers g ON p.message_id = g.message_id
WHERE
    strftime ('%Y', p.expiry) = ?
    AND p.passed = 1
GROUP BY
    g.user_id
ORDER BY
    total_points DESC;
