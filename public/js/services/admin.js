angular.module('scionApp')
    .factory('adminService', ["$http", "$q", "$httpParamSerializer", function ($http, $q, $httpParamSerializer) {
        return {
            adminPageData: function () {
                return $http.get('/api/adminPageData').then(function (response) {
                    console.log(response);
                    return response.data;
                });
            },
            sendInvitations: function (invitations) {
                console.log(angular.toJson(invitations));
                return $http.post('/api/sendInvitations', angular.toJson(invitations)).then(function (response) {
                    console.log(response);
                    return response.data;
                });
            },
            loadUsers: function () { // load all unactivated but verified users from the database
                return $http.get('/api/loadUnactivatedUsers');
            },
            activateUser: function (email) { // activate the user with the given email address
               return $http({
                    method: 'POST',
                    url: '/api/activateUser',
                    data: $httpParamSerializer({ 'email': email }),
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8' }
                })
            }
        };
    }]);
