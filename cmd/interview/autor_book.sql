CREATE TABLE Author
(
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE Book
(
    id    SERIAL PRIMARY KEY,
    title TEXT NOT NULL
);

CREATE TABLE Book_Author
( -- Многие ко многим
    book_id   INT REFERENCES Book (id) ON DELETE CASCADE,
    author_id INT REFERENCES Author (id) ON DELETE CASCADE,
    PRIMARY KEY (book_id, author_id)
);

CREATE TABLE Reader
(
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE BorrowedBook
( -- Учитываем, кто взял книгу
    id          SERIAL PRIMARY KEY,
    book_id     INT UNIQUE REFERENCES Book (id) ON DELETE CASCADE,
    reader_id   INT REFERENCES Reader (id) ON DELETE CASCADE,
    borrowed_at TIMESTAMP DEFAULT NOW()
);


select b.title
from Book b
         join borrowedbook bb on b.id = bb.book_id;

select b.title
from borrowedbook bor
         left join book_author ba on bor.book_id = ba.book_id
         left join book b on ba.book_id = b.id;
where bb.book_id is null
group by b.id
having count(ba.author_id) > 3;

select a.name, count(bb.book_id) as cnt
from Author a
         join book_author ba on a.id = ba.author_id
         join book b on ba.book_id = b.id
         join BorrowedBook BB on b.id = BB.book_id
group by a.id limit 3;












