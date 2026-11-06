CREATE INDEX search_transaction_hash ON transactions (network, hash COLLATE "C");

CREATE INDEX search_account_address ON transaction_accounts (network, address COLLATE "C");

CREATE INDEX search_contract_address ON contracts (network, contract_id COLLATE "C");
