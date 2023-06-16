const core = require('@actions/core')

try {
	const aud = core.getInput('aud')
	console.log(`hello future audience ${aud}`)
} catch (err) {
	core.setFailed(err.message)
}
