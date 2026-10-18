// Sends every request on golangagents.com (and www) to the same path and
// query on https://golang-agents.com with a permanent redirect.
export default {
  fetch(request) {
    const url = new URL(request.url);
    return Response.redirect(`https://golang-agents.com${url.pathname}${url.search}`, 301);
  },
};
