const express = require('express')
const app = express()
const path = process.env.REDIRECT_URL
let urls = {}
app.get('/metrics', (req, res) => {
    let metrics = "# HELP deploy_redirects amount of redirects on a given path.\n" +
        "# TYPE deploy_redirects counter\n"
    for (let [key, value] of Object.entries(urls)) {
        metrics += `deploy_redirects{path="${key}",service="deploy-redirector"} ${value}\n`
    }
    res.end(metrics)
})
app.get('*', (req, res) => {
    if (! urls[req.url]) {
        urls[req.url] = 0
    }
    urls[req.url] += 1
    res.redirect(301, path)
})

app.listen(8080)
