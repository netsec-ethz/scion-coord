angular.module('scionApp')
    .factory('adminService', ["$http", "$q", function($http, $q) {
        return {
            adminPageData: function () {
                return $http.get('/api/adminPageData').then(function (response) {
                    console.log(response);
                    return response.data;
                });
            },
        };
    }]);
