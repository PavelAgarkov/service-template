-- +goose Up
-- +goose StatementBegin
create table if not exists user_p
(
    id bigserial
        primary key
);

alter table user_p
    owner to habrpguser;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
