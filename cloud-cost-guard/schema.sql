CREATE TABLE IF NOT EXISTS daily_costs (
  date        date        NOT NULL,
  service     text        NOT NULL,
  amount      numeric(12,2) NOT NULL,
  currency    text        NOT NULL DEFAULT 'USD',
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (date, service)
);

CREATE TABLE IF NOT EXISTS alerts (
  id          bigserial   PRIMARY KEY,
  date        date        NOT NULL,
  service     text        NOT NULL,
  type        text        NOT NULL,
  message     text        NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
