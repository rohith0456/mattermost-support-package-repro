package generator

// generateLDIF writes ldap/users.ldif — a ready-to-load LDIF file with
// 8 test users and 4 groups under dc=repro,dc=local.
//
// Load into the running container with:
//
//	make ldap-users
//
// or manually:
//
//	docker compose exec openldap ldapadd -x \
//	  -D "cn=admin,dc=repro,dc=local" -w "ldap_admin_local_repro_only" \
//	  -f /ldap/users.ldif
func (g *Generator) generateLDIF() (string, error) {
	content := `# ============================================================
# mm-repro LDAP test users — dc=repro,dc=local
#
# Load with:  make ldap-users
# All users   password: Repro1234!
# ============================================================

# ── Organisational units ──────────────────────────────────────

dn: ou=people,dc=repro,dc=local
objectClass: organizationalUnit
ou: people

dn: ou=groups,dc=repro,dc=local
objectClass: organizationalUnit
ou: groups

# ── Users ─────────────────────────────────────────────────────

dn: uid=alice.johnson,ou=people,dc=repro,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
uid: alice.johnson
cn: Alice Johnson
sn: Johnson
givenName: Alice
mail: alice.johnson@repro.local
userPassword: Repro1234!
uidNumber: 1001
gidNumber: 1001
homeDirectory: /home/alice.johnson
loginShell: /bin/bash
title: Developer

dn: uid=bob.smith,ou=people,dc=repro,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
uid: bob.smith
cn: Bob Smith
sn: Smith
givenName: Bob
mail: bob.smith@repro.local
userPassword: Repro1234!
uidNumber: 1002
gidNumber: 1002
homeDirectory: /home/bob.smith
loginShell: /bin/bash
title: Developer

dn: uid=carol.white,ou=people,dc=repro,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
uid: carol.white
cn: Carol White
sn: White
givenName: Carol
mail: carol.white@repro.local
userPassword: Repro1234!
uidNumber: 1003
gidNumber: 1003
homeDirectory: /home/carol.white
loginShell: /bin/bash
title: Team Lead

dn: uid=dave.brown,ou=people,dc=repro,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
uid: dave.brown
cn: Dave Brown
sn: Brown
givenName: Dave
mail: dave.brown@repro.local
userPassword: Repro1234!
uidNumber: 1004
gidNumber: 1004
homeDirectory: /home/dave.brown
loginShell: /bin/bash
title: Designer

dn: uid=eve.davis,ou=people,dc=repro,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
uid: eve.davis
cn: Eve Davis
sn: Davis
givenName: Eve
mail: eve.davis@repro.local
userPassword: Repro1234!
uidNumber: 1005
gidNumber: 1005
homeDirectory: /home/eve.davis
loginShell: /bin/bash
title: QA Engineer

dn: uid=frank.miller,ou=people,dc=repro,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
uid: frank.miller
cn: Frank Miller
sn: Miller
givenName: Frank
mail: frank.miller@repro.local
userPassword: Repro1234!
uidNumber: 1006
gidNumber: 1006
homeDirectory: /home/frank.miller
loginShell: /bin/bash
title: Support Engineer

dn: uid=grace.wilson,ou=people,dc=repro,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
uid: grace.wilson
cn: Grace Wilson
sn: Wilson
givenName: Grace
mail: grace.wilson@repro.local
userPassword: Repro1234!
uidNumber: 1007
gidNumber: 1007
homeDirectory: /home/grace.wilson
loginShell: /bin/bash
title: Project Manager

dn: uid=henry.moore,ou=people,dc=repro,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
uid: henry.moore
cn: Henry Moore
sn: Moore
givenName: Henry
mail: henry.moore@repro.local
userPassword: Repro1234!
uidNumber: 1008
gidNumber: 1008
homeDirectory: /home/henry.moore
loginShell: /bin/bash
title: System Admin

# ── Groups ────────────────────────────────────────────────────

dn: cn=staff,ou=groups,dc=repro,dc=local
objectClass: groupOfNames
objectClass: top
cn: staff
member: uid=alice.johnson,ou=people,dc=repro,dc=local
member: uid=bob.smith,ou=people,dc=repro,dc=local
member: uid=carol.white,ou=people,dc=repro,dc=local
member: uid=dave.brown,ou=people,dc=repro,dc=local
member: uid=eve.davis,ou=people,dc=repro,dc=local
member: uid=frank.miller,ou=people,dc=repro,dc=local
member: uid=grace.wilson,ou=people,dc=repro,dc=local
member: uid=henry.moore,ou=people,dc=repro,dc=local

dn: cn=developers,ou=groups,dc=repro,dc=local
objectClass: groupOfNames
objectClass: top
cn: developers
member: uid=alice.johnson,ou=people,dc=repro,dc=local
member: uid=bob.smith,ou=people,dc=repro,dc=local
member: uid=carol.white,ou=people,dc=repro,dc=local

dn: cn=support,ou=groups,dc=repro,dc=local
objectClass: groupOfNames
objectClass: top
cn: support
member: uid=eve.davis,ou=people,dc=repro,dc=local
member: uid=frank.miller,ou=people,dc=repro,dc=local

dn: cn=management,ou=groups,dc=repro,dc=local
objectClass: groupOfNames
objectClass: top
cn: management
member: uid=carol.white,ou=people,dc=repro,dc=local
member: uid=grace.wilson,ou=people,dc=repro,dc=local
member: uid=henry.moore,ou=people,dc=repro,dc=local
`
	return g.writeFile("ldap/users.ldif", content)
}
