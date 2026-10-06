# Zeabur Deployment

- Project ID: `6ac4da755998d74f9f7394d2`
- Environment ID: `6ac4da7500d2ddfff3ef8520`
- Trend Studio Service ID: `6ac4da84cc325b8dc5d6232a`
- PostgreSQL Service ID: `6ac4da84cc325b8dc5d62329`
- Server ID: `6a40e0d4378dd7103f8b429d`
- URL: https://trend-studio-sakuralaaa.zeabur.app

Update these existing services in place. Preserve both persistent volumes; do not create duplicate services when updating an image. The application runs as user65532 and uses `serve` so API and Worker share one private volume. See `docs/zeabur.md` for first-volume initialization. Credentials belong only in Zeabur/private server configuration, never in this repository.
