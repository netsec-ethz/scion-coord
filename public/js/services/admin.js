angular.module('scionApp')
    .factory('adminService', ["$http", "$q", function ($http, $q) {
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
        };
    }]);
