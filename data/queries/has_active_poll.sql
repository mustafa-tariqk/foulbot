SELECT COUNT(*)
FROM polls
WHERE creator_id = ?
    AND passed IS NULL