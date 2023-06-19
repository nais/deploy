const core = require('@actions/core')

try {
	const aud = core.getInput('aud')
	console.log(`hello future audience ${aud}`)
	core.setOutput("result", "resulty things")
} catch (err) {
	core.setFailed(err.message)
}
