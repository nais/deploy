const core = require('@actions/core')

try {
	const aud = core.getInput('aud')
	console.log(`hello future audience ${aud}`)
	core.getIDToken(aud).then(token => {
		const buf = new Buffer(token)
		const tokenb64 = buf.toString('base64')
		core.setOutput('result', tokenb64)
	})
} catch (err) {
	core.setFailed(err.message)
}
