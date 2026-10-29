//go:build darwin

package providerusage

// The JXA bridge inspects metadata with UI disabled, without requesting the secret.
// Require security's trusted application and partition: a secret rewrite preserves
// the application ACL but replaces partition grants with those of the writer.
const claudeKeychainPreflightScript = `ObjC.import('Foundation');
ObjC.import('Security');
ObjC.bindFunction('SecTrustedApplicationValidateWithPath', ['int', ['id', 'char *']]);
function run(argv) {
  try {
    var c = ObjC.castRefToObject;
    function done(status, safe) { return JSON.stringify({status:status, safe:safe}); }
    if ($.SecKeychainSetUserInteractionAllowed(false) !== 0) return done(-25293, false);
    var query = $.NSMutableDictionary.alloc.init;
    query.setObjectForKey(c($.kSecClassGenericPassword), c($.kSecClass));
    query.setObjectForKey($(argv[0]), c($.kSecAttrService));
    query.setObjectForKey($(argv[1]), c($.kSecAttrAccount));
    query.setObjectForKey($.NSNumber.numberWithBool(true), c($.kSecReturnRef));
    query.setObjectForKey(c($.kSecUseAuthenticationUIFail), c($.kSecUseAuthenticationUI));
    var item = Ref();
    var status = $.SecItemCopyMatching(query, item);
    if (status !== 0) return done(status, false);

    var keychain = Ref();
    if ($.SecKeychainItemCopyKeychain(c(item[0]), keychain) !== 0) return done(-25293, false);

    var state = Ref();
    if ($.SecKeychainGetStatus(keychain[0], state) !== 0 || (state[0] & 1) === 0) return done(-25293, false);

    var access = Ref();
    if ($.SecKeychainItemCopyAccess(c(item[0]), access) !== 0) return done(-25293, false);

    var acls = Ref();
    if ($.SecAccessCopyACLList(access[0], acls) !== 0) return done(-25293, false);
    var list = c(acls[0]);
    var trusted = false, partition = false, decryptCount = 0, partitionCount = 0;
    for (var i = 0; i < list.count; i++) {
      var acl = list.objectAtIndex(i);
      var auths = ObjC.deepUnwrap(c($.SecACLCopyAuthorizations(acl)));
      var decrypt = auths.indexOf('ACLAuthorizationDecrypt') !== -1;
      var partitions = auths.indexOf('ACLAuthorizationPartitionID') !== -1;
      if (!decrypt && !partitions) continue;

      var apps = Ref(), description = Ref(), flags = Ref();
      if ($.SecACLCopyContents(acl, apps, description, flags) !== 0) return done(-25293, false);
      if (decrypt) {
        decryptCount++;
        if (flags[0] !== 0 || !apps[0]) return done(-25293, false);
        var paths = c(apps[0]);
        for (var j = 0; j < paths.count; j++) {
          var app = paths.objectAtIndex(j), data = Ref();
          if ($.SecTrustedApplicationCopyData(app, data) !== 0) continue;
          var path = ObjC.unwrap($.NSString.alloc.initWithDataEncoding(c(data[0]), $.NSUTF8StringEncoding));
          if (path.replace(/\0+$/, '') === '/usr/bin/security' &&
              $.SecTrustedApplicationValidateWithPath(app, '/usr/bin/security') === 0) trusted = true;
        }
      }
      if (partitions) {
        partitionCount++;
        var hex = ObjC.unwrap(c(description[0]));
        if (!/^(?:[0-9a-fA-F]{2})+$/.test(hex)) return done(-25293, false);
        var xml = '';
        for (var k = 0; k < hex.length; k += 2) xml += String.fromCharCode(parseInt(hex.substr(k, 2), 16));
        var plist = $.NSPropertyListSerialization.propertyListWithDataOptionsFormatError(
          $(xml).dataUsingEncoding($.NSUTF8StringEncoding), 0, null, null);
        var values = ObjC.deepUnwrap(plist);
        partition = values && Array.isArray(values.Partitions) && values.Partitions.indexOf('apple-tool:') !== -1;
      }
    }
    return done(0, trusted && partition && decryptCount === 1 && partitionCount === 1);
  } catch(e) { return JSON.stringify({status:-25293, safe:false}); }
}
`
