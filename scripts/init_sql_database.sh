
-- DROP FUNCTION public.update_timestamp();

CREATE OR REPLACE FUNCTION public.update_timestamp()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
       IF (TG_OP = 'INSERT' ) then 
          NEW.created = CURRENT_TIMESTAMP;
       END IF;
       NEW.updated_at = CURRENT_TIMESTAMP;
       RETURN NEW;
END;
$function$
;

-- Permissions

ALTER FUNCTION public.update_timestamp() OWNER TO postgres;
GRANT ALL ON FUNCTION public.update_timestamp() TO public;
GRANT ALL ON FUNCTION public.update_timestamp() TO postgres;
GRANT ALL ON FUNCTION public.update_timestamp() TO admin_album_role;

-- DROP FUNCTION public.update_timestamp_pic();

CREATE OR REPLACE FUNCTION public.update_timestamp_pic()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
       IF (TG_OP = 'INSERT' ) then 
          NEW.created = CURRENT_TIMESTAMP;
       END IF;
       NEW.updated_at = CURRENT_TIMESTAMP;
       RETURN NEW;
END;
$function$
;

-- Permissions

ALTER FUNCTION public.update_timestamp_pic() OWNER TO postgres;
GRANT ALL ON FUNCTION public.update_timestamp_pic() TO public;
GRANT ALL ON FUNCTION public.update_timestamp_pic() TO postgres;
GRANT ALL ON FUNCTION public.update_timestamp_pic() TO admin_album_role;

# create albumpictures
CREATE TABLE public.albumpictures (
	"index" public."uint4" NOT NULL,
	albumid int4 NOT NULL,
	"name" varchar(255) NULL,
	description varchar(255) NULL,
	checksumpicture varchar(42) NULL,
	mimetype varchar(150) NULL,
	fill varchar(20) NULL,
	skiptime public."uint4" NULL,
	height public."uint4" NULL,
	width public."uint4" NULL,
	created timestamp NULL,
	updated_at timestamp NULL,
	CONSTRAINT albumpictures_albumid_fkey FOREIGN KEY (albumid) REFERENCES public.albums(id) ON DELETE RESTRICT ON UPDATE RESTRICT
);

-- Table Triggers

create trigger update_timestamp before
insert
    or
update
    on
    public.albumpictures for each row execute function update_timestamp();

-- Permissions

ALTER TABLE public.albumpictures OWNER TO postgres;
GRANT ALL ON TABLE public.albumpictures TO postgres;
GRANT ALL ON TABLE public.albumpictures TO admin_album_role;
GRANT SELECT ON TABLE public.albumpictures TO read_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.albumpictures TO anja;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.albumpictures TO tkn WITH GRANT OPTION;

# create albums
CREATE TABLE public.albums (
	id serial4 NOT NULL,
	"type" varchar(32) DEFAULT ''::character varying NOT NULL,
	"key" varchar(40) DEFAULT ''::character varying NOT NULL,
	directory varchar(255) DEFAULT ''::character varying NOT NULL,
	title varchar(255) NOT NULL,
	description varchar(255) DEFAULT ''::character varying NOT NULL,
	"option" varchar(10) DEFAULT ''::character varying NOT NULL,
	thumbnailhash varchar(42) DEFAULT ''::character varying NOT NULL,
	published timestamp NULL,
	created timestamp NULL,
	updated_at timestamp NULL,
	"locked" bool DEFAULT false NOT NULL,
	collection bool DEFAULT false NOT NULL,
	CONSTRAINT albums_pkey PRIMARY KEY (id),
	CONSTRAINT albums_title_key UNIQUE (title)
);

-- Table Triggers

create trigger update_timestamp before
insert
    or
update
    on
    public.albums for each row execute function update_timestamp();

-- Permissions

ALTER TABLE public.albums OWNER TO postgres;
GRANT ALL ON TABLE public.albums TO postgres;
GRANT ALL ON TABLE public.albums TO admin_album_role;
GRANT SELECT ON TABLE public.albums TO read_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.albums TO anja;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.albums TO tkn WITH GRANT OPTION;

# create audit_table

CREATE TABLE public.audit_table (
	id serial4 NOT NULL,
	triggered timestamp NULL,
	elapsed int4 NULL,
	serverhost varchar(255) NULL,
	"method" varchar(255) NULL,
	remoteaddr varchar(255) NULL,
	remotehost varchar(255) NULL,
	"uuid" varchar(255) NULL,
	uri varchar(255) NULL,
	service varchar(2024) NULL,
	requestuser varchar(255) NULL,
	host varchar(255) NULL,
	tablename varchar(255) NULL,
	albumid int4 NULL,
	fields varchar(255) NULL,
	status varchar(255) NULL,
	CONSTRAINT audit_table_id_key UNIQUE (id)
);

-- Permissions

ALTER TABLE public.audit_table OWNER TO "admin";
GRANT ALL ON TABLE public.audit_table TO "admin";

# create batch repo

-- public.batch_repo definition

-- Drop table

-- DROP TABLE public.batch_repo;

CREATE TABLE public.batch_repo (
	"name" varchar(255) NOT NULL,
	query bytea NOT NULL,
	paramcount int4 NOT NULL,
	"database" varchar(255) NOT NULL
);

-- Permissions

ALTER TABLE public.batch_repo OWNER TO "admin";
GRANT ALL ON TABLE public.batch_repo TO "admin";

# create picturehash

CREATE TABLE public.picturehash (
	id bigserial NOT NULL,
	checksumpicture varchar(40) NOT NULL,
	hash numeric DEFAULT 0 NOT NULL,
	kind int4 NOT NULL,
	created timestamp NOT NULL,
	updated_at timestamp NOT NULL,
	averagehash numeric DEFAULT 0 NOT NULL,
	perceptionhash numeric DEFAULT 0 NOT NULL,
	differencehash numeric DEFAULT 0 NOT NULL,
	CONSTRAINT picturehash_pkey PRIMARY KEY (id),
	CONSTRAINT picturehash_unique UNIQUE (checksumpicture)
);

-- Table Triggers

create trigger update_timestamp_pic before
insert
    or
update
    on
    public.picturehash for each row execute function update_timestamp_pic();

-- Permissions

ALTER TABLE public.picturehash OWNER TO postgres;
GRANT ALL ON TABLE public.picturehash TO postgres;
GRANT SELECT ON TABLE public.picturehash TO read_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturehash TO admin_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturehash TO anja;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturehash TO tkn WITH GRANT OPTION;

# create picturelocations

CREATE TABLE public.picturelocations (
	checksumpicture varchar(40) NOT NULL,
	picturename varchar(255) DEFAULT ''::character varying NOT NULL,
	picturehost varchar(255) DEFAULT ''::character varying NOT NULL,
	picturedirectory varchar(255) DEFAULT ''::character varying NOT NULL,
	created timestamp NULL,
	updated_at timestamp NULL,
	CONSTRAINT picturelocations_checksumpicture_fkey FOREIGN KEY (checksumpicture) REFERENCES public.pictures(checksumpicture) ON DELETE RESTRICT ON UPDATE RESTRICT
);

-- Table Triggers

create trigger update_timestamp before
insert
    or
update
    on
    public.picturelocations for each row execute function update_timestamp();

-- Permissions

ALTER TABLE public.picturelocations OWNER TO postgres;
GRANT ALL ON TABLE public.picturelocations TO postgres;
GRANT ALL ON TABLE public.picturelocations TO admin_album_role;
GRANT SELECT ON TABLE public.picturelocations TO read_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturelocations TO anja;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturelocations TO tkn WITH GRANT OPTION;

# create pictures

CREATE TABLE public.pictures (
	id serial4 NOT NULL,
	checksumpicture varchar(40) NOT NULL,
	sha256checksum varchar(64) NOT NULL,
	thumbnail bytea NULL,
	media bytea NULL,
	title varchar(255) NULL,
	fill bpchar(1) NULL,
	mimetype varchar(255) NULL,
	"options" varchar(255) NULL,
	height int4 NULL,
	width int4 NULL,
	exifmodel varchar(100) NULL,
	exifmake varchar(100) NULL,
	exiftaken timestamp NULL,
	exiforigtime timestamp NULL,
	exifxdimension int4 NULL,
	exifydimension int4 NULL,
	exiforientation bpchar(1) NULL,
	created timestamp NULL,
	updated_at timestamp NULL,
	markdelete bool DEFAULT false NOT NULL,
	exif bytea NULL,
	gpscoordinates varchar(100) NULL,
	gpslatitude float8 DEFAULT 0 NOT NULL,
	gpslongitude float8 DEFAULT 0 NOT NULL,
	CONSTRAINT pictures_checksumpicture_key UNIQUE (checksumpicture),
	CONSTRAINT pictures_pkey PRIMARY KEY (id),
	CONSTRAINT pictures_sha256checksum_key UNIQUE (sha256checksum)
);
CREATE INDEX pictures_exiforigtime_idx ON public.pictures USING btree (exiforigtime);
CREATE INDEX pictures_mimetype_idx ON public.pictures USING btree (mimetype);

-- Table Triggers

create trigger update_timestamp_pic before
insert
    or
update
    on
    public.pictures for each row execute function update_timestamp_pic();

-- Permissions

ALTER TABLE public.pictures OWNER TO postgres;
GRANT ALL ON TABLE public.pictures TO postgres;
GRANT ALL ON TABLE public.pictures TO admin_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.pictures TO anja;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.pictures TO tkn WITH GRANT OPTION;

# create picturetags

CREATE TABLE public.picturetags (
	checksumpicture varchar(40) NOT NULL,
	tagname varchar(255) DEFAULT ''::character varying NOT NULL,
	id serial4 NOT NULL,
	created timestamp NULL,
	updated_at timestamp NULL,
	CONSTRAINT picturetags_pkey PRIMARY KEY (id),
	CONSTRAINT picturetags_checksumpicture_fkey FOREIGN KEY (checksumpicture) REFERENCES public.pictures(checksumpicture) ON DELETE RESTRICT ON UPDATE RESTRICT
);
CREATE INDEX picturetags_checksumpicture_idx ON public.picturetags USING btree (checksumpicture);

-- Table Triggers

create trigger update_timestamp before
insert
    or
update
    on
    public.picturetags for each row execute function update_timestamp();

-- Permissions

ALTER TABLE public.picturetags OWNER TO postgres;
GRANT ALL ON TABLE public.picturetags TO postgres;
GRANT SELECT ON TABLE public.picturetags TO read_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturetags TO admin_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturetags TO anja;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturetags TO tkn WITH GRANT OPTION;

# create session_info

CREATE TABLE public.session_info (
	"name" varchar(255) NULL,
	"uuid" varchar(255) NOT NULL,
	"data" bytea NULL,
	created timestamp NULL,
	lastaccess timestamp NULL,
	invalidated timestamp NULL,
	CONSTRAINT session_info_pkey PRIMARY KEY (uuid)
);

-- Permissions

ALTER TABLE public.session_info OWNER TO "admin";
GRANT ALL ON TABLE public.session_info TO "admin";

# create user_access

CREATE TABLE public.user_access (
	accreditation varchar(255) NOT NULL,
	"privileges" varchar(255) NOT NULL,
	CONSTRAINT user_access_unique UNIQUE (privileges, accreditation)
);

-- Permissions

ALTER TABLE public.user_access OWNER TO "admin";
GRANT ALL ON TABLE public.user_access TO "admin";

# create user_info

CREATE TABLE public.user_info (
	"name" varchar(255) NOT NULL,
	email varchar(255) NULL,
	longname varchar(255) NULL,
	created timestamp NULL,
	lastlogin timestamp NULL,
	picture bytea NULL,
	"permission" varchar(255) NULL,
	administrator bit(1) NULL,
	CONSTRAINT user_info_pkey PRIMARY KEY (name)
);

-- Permissions

ALTER TABLE public.user_info OWNER TO "admin";
GRANT ALL ON TABLE public.user_info TO "admin";

-- public.audit source

CREATE OR REPLACE VIEW public.audit
AS SELECT at2.triggered,
    at2.method,
    at2.albumid,
    at2.uuid,
    at2.remotehost,
    at2.tablename,
    at2.uri,
    at2.fields,
    at2.requestuser,
    at2.host,
    at2.status,
    a.title
   FROM audit_table at2,
    albums a
  WHERE at2.albumid > 0 AND at2.albumid = a.id;

-- Permissions

ALTER TABLE public.audit OWNER TO "admin";
GRANT ALL ON TABLE public.audit TO "admin";

CREATE OR REPLACE VIEW public.audit_raw
AS SELECT at2.triggered,
    at2.method,
    at2.albumid,
    at2.uuid,
    at2.remotehost,
    at2.tablename,
    at2.uri,
    at2.fields,
    at2.requestuser,
    at2.host,
    at2.status,
    COALESCE(s.title, '<Not defined>'::character varying) AS title
   FROM audit_table at2
     FULL JOIN albums s ON at2.albumid = s.id;

-- Permissions

ALTER TABLE public.audit_raw OWNER TO "admin";
GRANT ALL ON TABLE public.audit_raw TO "admin";

-- public.picture source

CREATE OR REPLACE VIEW public.picture
AS SELECT media,
    thumbnail,
    checksumpicture,
    mimetype
   FROM pictures t1;

-- Permissions

ALTER TABLE public.picture OWNER TO "admin";
GRANT ALL ON TABLE public.picture TO "admin";
GRANT SELECT ON TABLE public.picture TO read_album_role;

-- public.valbums source

CREATE OR REPLACE VIEW public.valbums
AS SELECT id,
    title,
    published,
    thumbnailhash,
    locked,
    key
   FROM albums tn
  WHERE title::text <> 'Default Album'::text;

-- Permissions

ALTER TABLE public.valbums OWNER TO "admin";
GRANT ALL ON TABLE public.valbums TO "admin";
GRANT SELECT ON TABLE public.valbums TO admin_album_role;
GRANT SELECT ON TABLE public.valbums TO inge;

-- public.vavgpicdoublikates source

CREATE OR REPLACE VIEW public.vavgpicdoublikates
AS SELECT count(averagehash) AS count,
    averagehash
   FROM picturehash p
  WHERE (EXISTS ( SELECT 1
           FROM pictures pp
          WHERE pp.checksumpicture::text = p.checksumpicture::text AND pp.markdelete = false))
  GROUP BY averagehash
 HAVING count(averagehash) > 1
  ORDER BY (count(averagehash)) DESC
 LIMIT 25;

-- Permissions

ALTER TABLE public.vavgpicdoublikates OWNER TO "admin";
GRANT ALL ON TABLE public.vavgpicdoublikates TO "admin";

-- public.vdifpicdoublikates source

CREATE OR REPLACE VIEW public.vdifpicdoublikates
AS SELECT count(differencehash) AS count,
    differencehash
   FROM picturehash p
  WHERE (EXISTS ( SELECT 1
           FROM pictures pp
          WHERE pp.checksumpicture::text = p.checksumpicture::text AND pp.markdelete = false))
  GROUP BY differencehash
 HAVING count(differencehash) > 1
  ORDER BY (count(differencehash)) DESC
 LIMIT 25;

-- Permissions

ALTER TABLE public.vdifpicdoublikates OWNER TO "admin";
GRANT ALL ON TABLE public.vdifpicdoublikates TO "admin";

-- public.vperpicdoublikates source

CREATE OR REPLACE VIEW public.vperpicdoublikates
AS SELECT count(perceptionhash) AS count,
    perceptionhash
   FROM picturehash p
  WHERE (EXISTS ( SELECT 1
           FROM pictures pp
          WHERE pp.checksumpicture::text = p.checksumpicture::text AND pp.markdelete = false))
  GROUP BY perceptionhash
 HAVING count(perceptionhash) > 1
  ORDER BY (count(perceptionhash)) DESC
 LIMIT 25;

-- Permissions

ALTER TABLE public.vperpicdoublikates OWNER TO "admin";
GRANT ALL ON TABLE public.vperpicdoublikates TO "admin";

-- public.vpicmedia source

CREATE OR REPLACE VIEW public.vpicmedia
AS SELECT media,
    thumbnail,
    checksumpicture,
    mimetype
   FROM pictures t1;

-- Permissions

ALTER TABLE public.vpicmedia OWNER TO "admin";
GRANT ALL ON TABLE public.vpicmedia TO "admin";

-- public.vpicsearchtag source

CREATE OR REPLACE VIEW public.vpicsearchtag
AS SELECT p.checksumpicture,
    p.sha256checksum,
    p.options,
    p.exifxdimension,
    p.exifydimension,
    p.exiforientation,
    p.height,
    p.width,
    p.exifmodel,
    p.exifmake,
    p.exiforigtime,
    p.created,
    p.updated_at,
    p.exiftaken,
    p.mimetype,
    p.title,
    pt.tagname
   FROM pictures p,
    picturetags pt
  WHERE p.checksumpicture::text = pt.checksumpicture::text AND p.markdelete = false;

-- Permissions

ALTER TABLE public.vpicsearchtag OWNER TO "admin";
GRANT ALL ON TABLE public.vpicsearchtag TO "admin";
GRANT SELECT ON TABLE public.vpicsearchtag TO read_album_role;

-- public.vsearchpicdoublikates source

CREATE OR REPLACE VIEW public.vsearchpicdoublikates
AS SELECT count(hash) AS count,
    hash
   FROM picturehash p
  WHERE (EXISTS ( SELECT 1
           FROM pictures pp
          WHERE pp.checksumpicture::text = p.checksumpicture::text AND pp.markdelete = false))
  GROUP BY hash
 HAVING count(hash) > 1
  ORDER BY (count(hash)) DESC
 LIMIT 25;

-- Permissions

ALTER TABLE public.vsearchpicdoublikates OWNER TO "admin";
GRANT ALL ON TABLE public.vsearchpicdoublikates TO "admin";
GRANT SELECT ON TABLE public.vsearchpicdoublikates TO tkn;
GRANT SELECT ON TABLE public.vsearchpicdoublikates TO admin_album_role;
GRANT SELECT ON TABLE public.vsearchpicdoublikates TO read_album_role;

-- public.vsearchpicmetadata source

CREATE OR REPLACE VIEW public.vsearchpicmetadata
AS SELECT checksumpicture,
    sha256checksum,
    mimetype,
    options,
    exifxdimension,
    exifydimension,
    exiforientation,
    height,
    width,
    exifmodel,
    exifmake,
    exiforigtime,
    created,
    updated_at,
    exiftaken,
    title,
    count(*) OVER () AS total_count
   FROM pictures t1;

-- Permissions

ALTER TABLE public.vsearchpicmetadata OWNER TO "admin";
GRANT ALL ON TABLE public.vsearchpicmetadata TO "admin";

-- public.vsearchpicmetadatatags source

CREATE OR REPLACE VIEW public.vsearchpicmetadatatags
AS SELECT t1.checksumpicture,
    t1.sha256checksum,
    t1.mimetype,
    t1.options,
    t1.exifxdimension,
    t1.exifydimension,
    t1.exiforientation,
    t1.height,
    t1.width,
    t1.exifmodel,
    t1.exifmake,
    t1.exiforigtime,
    t1.created,
    t1.updated_at,
    t1.exiftaken,
    t1.title,
    string_agg(a.tagname::text, ','::text ORDER BY (a.tagname::text)) AS tags,
    count(*) OVER () AS total_count
   FROM pictures t1
     LEFT JOIN picturetags a USING (checksumpicture)
  GROUP BY t1.checksumpicture, t1.sha256checksum, t1.mimetype, t1.options, t1.exifxdimension, t1.exifydimension, t1.exiforientation, t1.height, t1.width, t1.exifmodel, t1.exifmake, t1.exiforigtime, t1.created, t1.updated_at, t1.exiftaken, t1.title;

-- Permissions

ALTER TABLE public.vsearchpicmetadatatags OWNER TO "admin";
GRANT ALL ON TABLE public.vsearchpicmetadatatags TO "admin";

-- public.vsearchpictags source

CREATE OR REPLACE VIEW public.vsearchpictags
AS SELECT checksumpicture,
    sha256checksum,
    mimetype,
    options,
    exifxdimension,
    exifydimension,
    exiforientation,
    height,
    width,
    exifmodel,
    exifmake,
    exiforigtime,
    created,
    updated_at,
    exiftaken,
    markdelete,
    title,
    gpslatitude,
    gpslongitude,
    ( SELECT string_agg(DISTINCT (''''::text || p.tagname::text) || ''''::text, ','::text) AS string_agg
           FROM picturetags p
          WHERE p.checksumpicture::text = t1.checksumpicture::text) AS tags,
    ( SELECT ph.perceptionhash
           FROM picturehash ph
          WHERE ph.checksumpicture::text = t1.checksumpicture::text
         LIMIT 1) AS hash,
    count(*) OVER () AS total_count
   FROM pictures t1;

-- Permissions

ALTER TABLE public.vsearchpictags OWNER TO "admin";
GRANT ALL ON TABLE public.vsearchpictags TO "admin";
GRANT SELECT ON TABLE public.vsearchpictags TO read_album_role;

CREATE UNIQUE INDEX audit_table_id_key ON public.audit_table USING btree (id);
CREATE UNIQUE INDEX user_access_unique ON public.user_access USING btree (privileges, accreditation);
CREATE UNIQUE INDEX picturehash_pkey ON public.picturehash USING btree (id);
CREATE UNIQUE INDEX picturehash_unique ON public.picturehash USING btree (checksumpicture);
CREATE INDEX picturetags_checksumpicture_idx ON public.picturetags USING btree (checksumpicture);
CREATE UNIQUE INDEX picturetags_pkey ON public.picturetags USING btree (id);
CREATE UNIQUE INDEX albums_pkey ON public.albums USING btree (id);
CREATE UNIQUE INDEX albums_title_key ON public.albums USING btree (title);
CREATE UNIQUE INDEX pictures_checksumpicture_key ON public.pictures USING btree (checksumpicture);
CREATE INDEX pictures_exiforigtime_idx ON public.pictures USING btree (exiforigtime);
CREATE INDEX pictures_mimetype_idx ON public.pictures USING btree (mimetype);
CREATE UNIQUE INDEX pictures_pkey ON public.pictures USING btree (id);
CREATE UNIQUE INDEX pictures_sha256checksum_key ON public.pictures USING btree (sha256checksum);
CREATE UNIQUE INDEX user_info_pkey ON public.user_info USING btree (name);
CREATE UNIQUE INDEX session_info_pkey ON public.session_info USING btree (uuid);

-- DROP ROLE admin_album_role;

CREATE ROLE admin_album_role WITH 
	NOSUPERUSER
	NOCREATEDB
	NOCREATEROLE
	INHERIT
	NOLOGIN
	NOREPLICATION
	NOBYPASSRLS
	CONNECTION LIMIT -1;

-- Permissions

GRANT DELETE, INSERT, UPDATE, TRIGGER, TRUNCATE, SELECT, REFERENCES ON TABLE public.albumpictures TO admin_album_role;
GRANT DELETE, INSERT, UPDATE, TRIGGER, TRUNCATE, SELECT, REFERENCES ON TABLE public.albums TO admin_album_role;
GRANT UPDATE, USAGE, SELECT ON SEQUENCE public.albums_id_seq TO admin_album_role;
GRANT SELECT ON TABLE public.audit TO admin_album_role;
GRANT SELECT ON TABLE public.audit_raw TO admin_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturehash TO admin_album_role;
GRANT DELETE, INSERT, UPDATE, TRIGGER, TRUNCATE, SELECT, REFERENCES ON TABLE public.picturelocations TO admin_album_role;
GRANT DELETE, INSERT, UPDATE, TRIGGER, TRUNCATE, SELECT, REFERENCES ON TABLE public.pictures TO admin_album_role;
GRANT UPDATE, USAGE, SELECT ON SEQUENCE public.pictures_id_seq TO admin_album_role;
GRANT DELETE, INSERT, UPDATE, SELECT ON TABLE public.picturetags TO admin_album_role;
GRANT CREATE, USAGE ON SCHEMA public TO admin_album_role;
GRANT EXECUTE ON FUNCTION public.update_timestamp() TO admin_album_role WITH GRANT OPTION;
GRANT EXECUTE ON FUNCTION public.update_timestamp_pic() TO admin_album_role WITH GRANT OPTION;
GRANT SELECT ON TABLE public.valbums TO admin_album_role;
GRANT SELECT ON TABLE public.vsearchpicdoublikates TO admin_album_role;

-- DROP ROLE read_album_role;

CREATE ROLE read_album_role WITH 
	NOSUPERUSER
	NOCREATEDB
	NOCREATEROLE
	INHERIT
	NOLOGIN
	NOREPLICATION
	NOBYPASSRLS
	CONNECTION LIMIT -1;

-- Permissions

GRANT SELECT ON TABLE public.albumpictures TO read_album_role;
GRANT SELECT ON TABLE public.albums TO read_album_role;
GRANT SELECT ON TABLE public.picture TO read_album_role;
GRANT SELECT ON TABLE public.picturehash TO read_album_role;
GRANT SELECT ON TABLE public.picturelocations TO read_album_role;
GRANT SELECT ON TABLE public.picturetags TO read_album_role;
GRANT SELECT ON TABLE public.vpicsearchtag TO read_album_role;
GRANT SELECT ON TABLE public.vsearchpicdoublikates TO read_album_role;
GRANT SELECT ON TABLE public.vsearchpictags TO read_album_role;

