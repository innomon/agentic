# RouterAuthAgent: Router Auth Agent

The `RouterAuthAgent` will route the users based on their roles.

## User Role Tool
Create a PgSQL dabase tool, that will store user (userID), channels and roles.

- user can have multiple roles and can have multile channels. a user with "ALL" have access to all channels.
- user database schema, will store, userID as key, and JSONB metadata having fields like, update timestamp, updated by and channel. A profile JSONB field will have channels access array, userID, roles array. A status 
field will define Active/Pending/Suspended. 
- admin roles can never be set via any channels. It can only be pre-configured in config file.
- admins can create/delete/modify status, roles and channels.

**Admin**: users with Admin role can only be assigned from config, and not changed via database access. Admin have access to all channels, router will directly route it to admin subagent.

## Routing agent logic

The routing agent config, will have subagents, and roles to subagent mapping for routing.
If a user has multiple roles, then the context query will determine the routing.

For example, if a user has a farmer and seller profile, then the routing to a specific agent will be decided by the context of the query.

The agent will first retrieve the user profile from the tool, if the user is not present it will anonymous profile, if the anonymous profile is not mapped to any subagent, it will reject with a polite message.

if the user has multiple roles matching with multiple routes, then router will consult a disamgiguation subagent, passing the user id, roles options along with context, the  subagent will return the most appropiate role for the context. Thereafter the routing agent will route it to the apporiate aubagent.
