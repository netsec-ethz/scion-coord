angular.module('scionApp')
    .factory('userService', ["$http", function($http) {
    var userService = {
        var user = {};

        return {
            get: function () {
                return user;
            },
            set: function(value) {
                user = value;
            }
        };
    };
    return userService;
}]);
