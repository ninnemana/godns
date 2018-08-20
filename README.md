# Google Dynamic DNS Client

Pings the Google Dynamic DNS Server using the provided credentials on the
defined interval.

## Documentation for the Google Service

```
Dynamic DNS client software automatically updates your dynamic DNS record. You can perform updates manually with the API by making making a POST request (GET is also allowed) to the following url:
https://domains.google.com/nic/update

The API requires HTTPS. Here’s an example request:
https://username:password@domains.google.com/nic/update?hostname=subdomain.yourdomain.com&myip=1.2.3.4

Note: You must set a user agent in your request as well. Web browsers will generally add this for you when testing via the above url. In any case, the final HTTP request sent to our servers should look something like this:

Example HTTP query:
POST /nic/update?hostname=subdomain.yourdomain.com&myip=1.2.3.4 HTTP/1.1
Host: domains.google.com
Authorization: Basic base64-encoded-auth-string User-Agent: Chrome/41.0 your_email@yourdomain.com

Request Parameters:

Parameter	Required/Optional	Description
username:password	Required	The generated username and password associated with the host that is to be updated.
hostname	Required	The hostname to be updated.
myip	Optional
(Required if you have an IPv6 address)	The IP address to which the host will be set. If not supplied, we’ll use the IP of the agent that sent the request.
Note: Because the address must be an IPv4 address, myip is required if your agent uses an IPv6 address. You can check your agent’s IP address by going to https://domains.google.com/checkip.

offline	Optional	Sets the current host to offline status. If an update request is performed on an offline host, the host is removed from the offline state.
Allowed values are
yes
no
One of the following responses will be returned after the request is processed.

Please ensure you interpret the response correctly, or you risk having your client blocked from our system.
Response	Status	Description
good 1.2.3.4	Success	The update was successful. Followed by a space and the updated IP address. You should not attempt another update until your IP address changes.
nochg 1.2.3.4	Success	The supplied IP address is already set for this host. You should not attempt another update until your IP address changes.
nohost	Error	The hostname does not exist, or does not have Dynamic DNS enabled.
badauth	Error	The username / password combination is not valid for the specified host.
notfqdn	Error	The supplied hostname is not a valid fully-qualified domain name.
badagent	Error	Your Dynamic DNS client is making bad requests. Ensure the user agent is set in the request, and that you’re only attempting to set an IPv4 address. IPv6 is not supported.
abuse	Error	Dynamic DNS access for the hostname has been blocked due to failure to interpret previous responses correctly.
911	Error	An error happened on our end. Wait 5 minutes and retry.
```

