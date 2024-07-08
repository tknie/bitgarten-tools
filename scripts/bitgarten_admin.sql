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

ALTER TABLE public.pictures ADD markdelete boolean DEFAULT false NOT NULL;
ALTER TABLE public.pictures ADD exif bytea NULL;
ALTER TABLE public.pictures ADD gpscoordinates varchar(100) NULL;
ALTER TABLE public.pictures ADD gpslatitude float8 NULL;
ALTER TABLE public.pictures ADD gpslongitude float8 NULL;

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
