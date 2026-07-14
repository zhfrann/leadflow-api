CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT users_email_not_blank
        CHECK (BTRIM(email) <> ''),

    CONSTRAINT users_email_trimmed
        CHECK (email = BTRIM(email)),

    CONSTRAINT users_email_length
        CHECK (CHAR_LENGTH(email) <= 320),

    CONSTRAINT users_password_hash_not_blank
        CHECK (BTRIM(password_hash) <> '')
);

CREATE UNIQUE INDEX users_email_unique_idx
    ON users (LOWER(email));


CREATE TABLE contacts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT,
    company TEXT,
    status TEXT NOT NULL DEFAULT 'NEW',
    source TEXT NOT NULL DEFAULT 'OTHER',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT contacts_owner_id_foreign
        FOREIGN KEY (owner_id)
        REFERENCES users (id)
        ON DELETE CASCADE,

    CONSTRAINT contacts_name_not_blank
        CHECK (BTRIM(name) <> ''),

    CONSTRAINT contacts_name_length
        CHECK (CHAR_LENGTH(name) <= 200),

    CONSTRAINT contacts_email_not_blank
        CHECK (BTRIM(email) <> ''),

    CONSTRAINT contacts_email_trimmed
        CHECK (email = BTRIM(email)),

    CONSTRAINT contacts_email_length
        CHECK (CHAR_LENGTH(email) <= 320),

    CONSTRAINT contacts_phone_not_blank
        CHECK (
            phone IS NULL
            OR BTRIM(phone) <> ''
        ),

    CONSTRAINT contacts_company_not_blank
        CHECK (
            company IS NULL
            OR BTRIM(company) <> ''
        ),

    CONSTRAINT contacts_status_valid
        CHECK (
            status IN (
                'NEW',
                'CONTACTED',
                'QUALIFIED',
                'CUSTOMER',
                'LOST'
            )
        ),

    CONSTRAINT contacts_source_valid
        CHECK (
            source IN (
                'WEBSITE',
                'REFERRAL',
                'SOCIAL_MEDIA',
                'OTHER'
            )
        )
);

CREATE UNIQUE INDEX contacts_owner_email_active_unique_idx
    ON contacts (owner_id, LOWER(email))
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX contacts_owner_phone_active_unique_idx
    ON contacts (owner_id, phone)
    WHERE phone IS NOT NULL
      AND deleted_at IS NULL;

CREATE INDEX contacts_owner_created_at_idx
    ON contacts (owner_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX contacts_owner_status_created_at_idx
    ON contacts (
        owner_id,
        status,
        created_at DESC,
        id DESC
    )
    WHERE deleted_at IS NULL;


CREATE TABLE email_outbox (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_type TEXT NOT NULL,
    recipient_email TEXT NOT NULL,
    template_name TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT,
    processing_started_at TIMESTAMPTZ,
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT email_outbox_event_type_valid
        CHECK (
            event_type IN (
                'USER_REGISTERED',
                'CONTACT_BECAME_CUSTOMER'
            )
        ),

    CONSTRAINT email_outbox_recipient_not_blank
        CHECK (BTRIM(recipient_email) <> ''),

    CONSTRAINT email_outbox_template_name_valid
        CHECK (
            template_name IN (
                'welcome',
                'contact_became_customer'
            )
        ),

    CONSTRAINT email_outbox_payload_object
        CHECK (JSONB_TYPEOF(payload) = 'object'),

    CONSTRAINT email_outbox_status_valid
        CHECK (
            status IN (
                'PENDING',
                'PROCESSING',
                'SENT',
                'FAILED'
            )
        ),

    CONSTRAINT email_outbox_attempt_count_non_negative
        CHECK (attempt_count >= 0)
);

CREATE INDEX email_outbox_pending_idx
    ON email_outbox (
        next_attempt_at,
        created_at,
        id
    )
    WHERE status = 'PENDING';

CREATE INDEX email_outbox_processing_idx
    ON email_outbox (
        processing_started_at,
        id
    )
    WHERE status = 'PROCESSING';
