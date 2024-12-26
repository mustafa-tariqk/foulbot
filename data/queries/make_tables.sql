CREATE TABLE IF NOT EXISTS "polls" (
    "channel_id" TEXT NOT NULL,
    "message_id" TEXT NOT NULL,
    "creator_id" TEXT NOT NULL,
    "points" INTEGER NOT NULL,
    "reason" TEXT NOT NULL,
    "expiry" TEXT NOT NULL,
    "passed" INTEGER,
    PRIMARY KEY ("channel_id", "message_id")
);

CREATE TABLE IF NOT EXISTS "gainers" (
    "channel_id" TEXT NOT NULL,
    "message_id" TEXT NOT NULL,
    "user_id" TEXT NOT NULL,
    PRIMARY KEY ("channel_id", "message_id", "user_id"),
    FOREIGN KEY ("channel_id") REFERENCES "polls" ("channel_id"),
    FOREIGN KEY ("message_id") REFERENCES "polls" ("message_id")
);

CREATE TABLE IF NOT EXISTS "votes" (
    "channel_id" TEXT NOT NULL,
    "message_id" TEXT NOT NULL,
    "user_id" TEXT NOT NULL,
    "value" INTEGER NOT NULL,
    PRIMARY KEY ("channel_id", "message_id", "user_id"),
    FOREIGN KEY ("channel_id") REFERENCES "polls" ("channel_id"),
    FOREIGN KEY ("message_id") REFERENCES "polls" ("message_id")
);
