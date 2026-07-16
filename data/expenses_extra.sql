-- Фиксированные расходы дополнительных участников (генерируется gendemo)
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Аренда цеха и точки', 250000, 3, 'rent' FROM participants WHERE account_id = 'ACC-003'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Зарплаты: расчёт', 150000, 5, 'salary' FROM participants WHERE account_id = 'ACC-003'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Зарплаты: аванс', 150000, 20, 'salary' FROM participants WHERE account_id = 'ACC-003'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Мука и сырьё', 100000, 12, 'supplies' FROM participants WHERE account_id = 'ACC-003'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Коммуналка', 40000, 25, 'utilities' FROM participants WHERE account_id = 'ACC-003'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Аренда салона', 200000, 19, 'rent' FROM participants WHERE account_id = 'ACC-004'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Зарплаты мастеров', 220000, 10, 'salary' FROM participants WHERE account_id = 'ACC-004'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Косметика и расходники', 60000, 18, 'supplies' FROM participants WHERE account_id = 'ACC-004'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Коммуналка и интернет', 40000, 27, 'utilities' FROM participants WHERE account_id = 'ACC-004'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Аренда точки и парковка', 90000, 7, 'rent' FROM participants WHERE account_id = 'ACC-005'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Зарплата помощника', 120000, 20, 'salary' FROM participants WHERE account_id = 'ACC-005'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
INSERT INTO fixed_expenses (participant_id, description, amount, due_day_of_month, category)
SELECT id, 'Закупка продуктов', 50000, 14, 'supplies' FROM participants WHERE account_id = 'ACC-005'
ON CONFLICT (participant_id, description) DO UPDATE SET amount = EXCLUDED.amount, due_day_of_month = EXCLUDED.due_day_of_month, category = EXCLUDED.category;
